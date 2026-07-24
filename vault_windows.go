//go:build windows

package vault

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Windows implementation storing Generic credentials in the Windows
// Credential Manager via PowerShell P/Invoke on advapi32
// (CredRead/CredWrite/CredDelete). No CGO required.
//
// Exit codes and error detection are used instead of parsing command
// output, which is localized on non-English Windows.
//
// Note: Generic credential blobs are limited to 2560 bytes
// (CRED_MAX_CREDENTIAL_BLOB_SIZE). The value is base64 encoded and stored
// as UTF-16, so values up to roughly 960 raw bytes fit.

// credManType defines the advapi32 wrapper. Each operation appends a small
// tail that calls into it and exits with: 0 success, 2 not found, 1 error.
const credManType = `
$ErrorActionPreference = 'Stop'
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;

public static class VaultCred {
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    public struct CREDENTIAL {
        public uint Flags;
        public uint Type;
        public string TargetName;
        public string Comment;
        public System.Runtime.InteropServices.ComTypes.FILETIME LastWritten;
        public uint CredentialBlobSize;
        public IntPtr CredentialBlob;
        public uint Persist;
        public uint AttributeCount;
        public IntPtr Attributes;
        public string TargetAlias;
        public string UserName;
    }

    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool CredWriteW(ref CREDENTIAL credential, uint flags);

    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool CredReadW(string target, uint type, uint flags, out IntPtr credentialPtr);

    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool CredDeleteW(string target, uint type, uint flags);

    [DllImport("advapi32.dll")]
    private static extern void CredFree(IntPtr buffer);

    private const uint CRED_TYPE_GENERIC = 1;
    private const uint CRED_PERSIST_LOCAL_MACHINE = 2;
    private const int ERROR_NOT_FOUND = 1168;

    public static int Write(string target, string secret) {
        byte[] blob = System.Text.Encoding.Unicode.GetBytes(secret);
        IntPtr blobPtr = Marshal.AllocHGlobal(blob.Length);
        try {
            Marshal.Copy(blob, 0, blobPtr, blob.Length);
            CREDENTIAL cred = new CREDENTIAL();
            cred.Type = CRED_TYPE_GENERIC;
            cred.TargetName = target;
            cred.CredentialBlobSize = (uint)blob.Length;
            cred.CredentialBlob = blobPtr;
            cred.Persist = CRED_PERSIST_LOCAL_MACHINE;
            cred.UserName = "vault";
            return CredWriteW(ref cred, 0) ? 0 : 1;
        } finally {
            Marshal.FreeHGlobal(blobPtr);
        }
    }

    // Returns null when the credential does not exist; throws on other errors.
    public static string Read(string target) {
        IntPtr ptr;
        if (!CredReadW(target, CRED_TYPE_GENERIC, 0, out ptr)) {
            if (Marshal.GetLastWin32Error() == ERROR_NOT_FOUND) {
                return null;
            }
            throw new InvalidOperationException("CredRead failed: " + Marshal.GetLastWin32Error());
        }
        try {
            CREDENTIAL cred = (CREDENTIAL)Marshal.PtrToStructure(ptr, typeof(CREDENTIAL));
            byte[] blob = new byte[cred.CredentialBlobSize];
            if (cred.CredentialBlobSize > 0) {
                Marshal.Copy(cred.CredentialBlob, blob, 0, (int)cred.CredentialBlobSize);
            }
            return System.Text.Encoding.Unicode.GetString(blob);
        } finally {
            CredFree(ptr);
        }
    }

    public static int Delete(string target) {
        if (CredDeleteW(target, CRED_TYPE_GENERIC, 0)) {
            return 0;
        }
        return Marshal.GetLastWin32Error() == ERROR_NOT_FOUND ? 2 : 1;
    }
}
'@
`

const exitCodeNotFound = 2

// psQuote escapes s for use inside a single-quoted PowerShell string.
func psQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// runPS executes a PowerShell script and returns its stdout.
// notFound is true when the script exited with exitCodeNotFound.
func runPS(script string) (stdout string, notFound bool, err error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == exitCodeNotFound {
			return "", true, nil
		}
		return "", false, fmt.Errorf("vault: powershell failed: %s: %w", strings.TrimSpace(stderr.String()), err)
	}
	return out.String(), false, nil
}

func set(service, key string, value []byte) error {
	target := psQuote(service + "/" + key)
	encoded := base64.StdEncoding.EncodeToString(value)

	script := credManType + fmt.Sprintf("\nexit [VaultCred]::Write('%s', '%s')\n", target, encoded)

	if _, _, err := runPS(script); err != nil {
		return fmt.Errorf("vault: failed to set key: %w", err)
	}
	return nil
}

func get(service, key string) ([]byte, error) {
	target := psQuote(service + "/" + key)

	script := credManType + fmt.Sprintf(`
$secret = [VaultCred]::Read('%s')
if ($null -eq $secret) { exit 2 }
Write-Output $secret
exit 0
`, target)

	stdout, notFound, err := runPS(script)
	if err != nil {
		return nil, fmt.Errorf("vault: failed to get key: %w", err)
	}
	if notFound {
		return nil, ErrNotFound
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(stdout))
	if err != nil {
		return nil, fmt.Errorf("vault: failed to decode value: %w", err)
	}
	return decoded, nil
}

func del(service, key string) error {
	target := psQuote(service + "/" + key)

	script := credManType + fmt.Sprintf("\nexit [VaultCred]::Delete('%s')\n", target)

	_, notFound, err := runPS(script)
	if err != nil {
		return fmt.Errorf("vault: failed to delete key: %w", err)
	}
	if notFound {
		return ErrNotFound
	}
	return nil
}
