//go:build windows

package vault

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
)

// Windows implementation using PowerShell with DPAPI (Data Protection API)
// through the Windows Credential Manager. No CGO required.

func set(service, key string, value []byte) error {
	// Use PowerShell to store credential in Windows Credential Manager
	// The credential is stored as a Generic credential
	credName := service + "/" + key
	encodedValue := base64.StdEncoding.EncodeToString(value)

	// PowerShell script to add credential
	script := fmt.Sprintf(`
$credName = '%s'
$credValue = '%s'
$securePassword = ConvertTo-SecureString -String $credValue -AsPlainText -Force
$credential = New-Object System.Management.Automation.PSCredential($credName, $securePassword)

# Remove existing credential if it exists
try {
    cmdkey /delete:$credName 2>$null
} catch {}

# Add new credential using cmdkey
$bytes = [System.Text.Encoding]::UTF8.GetBytes($credValue)
cmdkey /generic:$credName /user:$credName /pass:$credValue
if ($LASTEXITCODE -ne 0) {
    exit 1
}
`, credName, encodedValue)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vault: failed to set key: %s", stderr.String())
	}

	return nil
}

func get(service, key string) ([]byte, error) {
	credName := service + "/" + key

	// PowerShell script to retrieve credential
	script := fmt.Sprintf(`
$output = cmdkey /list:"%s" 2>&1
if ($output -match "NONE") {
    exit 1
}

# Use .NET to read the credential
Add-Type -AssemblyName System.Runtime.InteropServices

$sig = @"
[DllImport("advapi32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
public static extern bool CredRead(string target, int type, int reservedFlag, out IntPtr credentialPtr);

[DllImport("advapi32.dll", SetLastError = true)]
public static extern bool CredFree(IntPtr cred);
"@

$advapi32 = Add-Type -MemberDefinition $sig -Namespace "ADVAPI32" -Name "Util" -PassThru

$credPtr = [IntPtr]::Zero
$result = $advapi32::CredRead("%s", 1, 0, [ref]$credPtr)

if (-not $result) {
    exit 1
}

$cred = [System.Runtime.InteropServices.Marshal]::PtrToStructure($credPtr, [Type][System.Runtime.InteropServices.ComTypes.CREDENTIAL])

# Read credential blob
$blob = New-Object byte[] $cred.CredentialBlobSize
[System.Runtime.InteropServices.Marshal]::Copy($cred.CredentialBlob, $blob, 0, $cred.CredentialBlobSize)
$password = [System.Text.Encoding]::Unicode.GetString($blob)

$advapi32::CredFree($credPtr)
Write-Output $password
`, credName, credName)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, ErrNotFound
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return nil, ErrNotFound
	}

	decoded, err := base64.StdEncoding.DecodeString(result)
	if err != nil {
		return nil, fmt.Errorf("vault: failed to decode value: %w", err)
	}

	return decoded, nil
}

func del(service, key string) error {
	credName := service + "/" + key

	cmd := exec.Command("cmdkey", "/delete:"+credName)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errStr := stderr.String()
		if strings.Contains(strings.ToLower(errStr), "not found") ||
			strings.Contains(strings.ToLower(errStr), "none") {
			return ErrNotFound
		}
		return fmt.Errorf("vault: failed to delete key: %s", errStr)
	}

	return nil
}
