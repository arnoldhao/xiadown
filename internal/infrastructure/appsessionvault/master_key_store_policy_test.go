package appsessionvault

import (
	"os"
	"strings"
	"testing"
)

func TestDarwinResetKeyDeletionIsNonInteractive(t *testing.T) {
	sourceBytes, err := os.ReadFile("master_key_store_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	currentStart := strings.Index(source, "static int sessionVaultDeleteMasterKey(")
	legacyStart := strings.Index(source, "static int sessionVaultDeleteLegacyService(")
	preambleEnd := strings.Index(source, "*/")
	if currentStart < 0 || legacyStart <= currentStart || preambleEnd <= legacyStart {
		t.Fatalf("Darwin reset deletion functions are missing: current=%d legacy=%d end=%d", currentStart, legacyStart, preambleEnd)
	}
	for name, functionSource := range map[string]string{
		"current": source[currentStart:legacyStart],
		"legacy":  source[legacyStart:preambleEnd],
	} {
		if !strings.Contains(functionSource, "interactionNotAllowed = YES") ||
			!strings.Contains(functionSource, "kSecUseAuthenticationContext: authenticationContext") {
			t.Fatalf("%s Keychain deletion can request UI", name)
		}
	}
	if !strings.Contains(source, "C.CString(legacyKeychainService)") || legacyKeychainService != "com.dreamapp.xiadown.connector-app-session" {
		t.Fatal("Darwin application reset does not target the legacy App Session service")
	}
}

func TestDarwinMasterKeyInventoryDoesNotReadSecretData(t *testing.T) {
	sourceBytes, err := os.ReadFile("master_key_store_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	start := strings.Index(source, "static int sessionVaultInspectMasterKey(")
	end := strings.Index(source, "static int sessionVaultStoreMasterKey(")
	if start < 0 || end <= start {
		t.Fatalf("Darwin master-key inventory function is missing: start=%d end=%d", start, end)
	}
	functionSource := source[start:end]
	if strings.Contains(functionSource, "kSecReturnData:") {
		t.Fatal("Data Management inventory reads Session Vault key material")
	}
	if !strings.Contains(functionSource, "kSecReturnAttributes: @YES") ||
		!strings.Contains(functionSource, "interactionNotAllowed = YES") {
		t.Fatal("Data Management inventory is not attributes-only and non-interactive")
	}
}

func TestDarwinMasterKeyUsesClassicKeychainWithoutInteractiveOperations(t *testing.T) {
	sourceBytes, err := os.ReadFile("master_key_store_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	if strings.Contains(source, "kSecUseDataProtectionKeychain") {
		t.Fatal("Session Vault master key depends on the provisioning-only Data Protection Keychain")
	}
	functionNames := []string{
		"sessionVaultLoadMasterKey",
		"sessionVaultInspectMasterKey",
		"sessionVaultStoreMasterKey",
		"sessionVaultDeleteMasterKey",
	}
	for index, name := range functionNames {
		start := strings.Index(source, "static int "+name+"(")
		end := strings.Index(source, "*/")
		if index+1 < len(functionNames) {
			end = strings.Index(source, "static int "+functionNames[index+1]+"(")
		}
		if start < 0 || end <= start {
			t.Fatalf("Darwin Keychain function %s is missing", name)
		}
		functionSource := source[start:end]
		if !strings.Contains(functionSource, "interactionNotAllowed = YES") ||
			!strings.Contains(functionSource, "kSecUseAuthenticationContext: authenticationContext") {
			t.Fatalf("Darwin Keychain function %s may show authentication UI", name)
		}
	}
	storeStart := strings.Index(source, "static int sessionVaultStoreMasterKey(")
	deleteStart := strings.Index(source, "static int sessionVaultDeleteMasterKey(")
	storeSource := source[storeStart:deleteStart]
	if !strings.Contains(storeSource, "kSecAttrAccessibleWhenUnlocked") ||
		strings.Contains(storeSource, "kSecAttrAccessibleWhenUnlockedThisDeviceOnly") ||
		!strings.Contains(storeSource, "kSecAttrSynchronizable: @NO") {
		t.Fatal("Session Vault master key is not explicitly local to the classic login Keychain")
	}
}
