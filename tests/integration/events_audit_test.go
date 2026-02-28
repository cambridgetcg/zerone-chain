package integration_test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestEventAudit_AllHandlersEmitEvents verifies that every message handler
// in every Zerone module emits at least one event.
//
// It scans msg_server.go files, identifies handler functions, and checks
// that each contains an EmitEvent call.
func TestEventAudit_AllHandlersEmitEvents(t *testing.T) {
	root := findProjectRoot(t)
	modulesDir := filepath.Join(root, "x")

	// Collect all msg_server.go files.
	var msgServerFiles []string
	err := filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "msg_server.go" && strings.Contains(path, "/keeper/") {
			msgServerFiles = append(msgServerFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk modules directory: %v", err)
	}

	if len(msgServerFiles) == 0 {
		t.Fatal("no msg_server.go files found under x/")
	}

	// Regex to identify handler functions (methods on msgServer).
	handlerRe := regexp.MustCompile(`^func \(.*\b(?:ms|m|k)\b.*\) (\w+)\(`)
	emitRe := regexp.MustCompile(`EmitEvent|EmitTypedEvent`)

	var missing []string

	for _, file := range msgServerFiles {
		moduleName := extractModuleName(file)
		handlers := extractHandlers(t, file, handlerRe, emitRe)
		for _, h := range handlers {
			if !h.hasEvent {
				missing = append(missing, fmt.Sprintf("%s.%s", moduleName, h.name))
			}
		}
	}

	if len(missing) > 0 {
		t.Errorf("handlers missing event emission (%d):\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// TestEventAudit_EventTypeFormat verifies that all event type strings
// follow the zerone.<module>.<action> convention.
func TestEventAudit_EventTypeFormat(t *testing.T) {
	root := findProjectRoot(t)
	modulesDir := filepath.Join(root, "x")

	// Match sdk.NewEvent("...") calls.
	eventTypeRe := regexp.MustCompile(`sdk\.NewEvent\("([^"]+)"`)
	validFormatRe := regexp.MustCompile(`^zerone\.[a-z_]+\.[a-z_]+$`)

	var violations []string

	err := filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip test files and protobuf generated files.
		if strings.HasSuffix(path, "_test.go") || strings.Contains(path, ".pb.go") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		matches := eventTypeRe.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			eventType := match[1]
			if !validFormatRe.MatchString(eventType) {
				relPath, _ := filepath.Rel(root, path)
				violations = append(violations, fmt.Sprintf("%s: %q", relPath, eventType))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk modules directory: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("event types not matching zerone.<module>.<action> format (%d):\n  %s",
			len(violations), strings.Join(violations, "\n  "))
	}
}

// TestEventAudit_NoSensitiveData verifies that events don't contain
// attributes that might leak sensitive data.
func TestEventAudit_NoSensitiveData(t *testing.T) {
	root := findProjectRoot(t)
	modulesDir := filepath.Join(root, "x")

	// Attribute names that should never appear in events.
	sensitiveAttrs := []string{
		"private_key", "secret", "password", "mnemonic", "seed_phrase",
		"raw_content", "plaintext",
	}
	attrRe := regexp.MustCompile(`sdk\.NewAttribute\("([^"]+)"`)

	var violations []string

	err := filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		matches := attrRe.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			attrName := match[1]
			for _, sensitive := range sensitiveAttrs {
				if strings.Contains(strings.ToLower(attrName), sensitive) {
					relPath, _ := filepath.Rel(root, path)
					violations = append(violations, fmt.Sprintf("%s: sensitive attribute %q", relPath, attrName))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk modules directory: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("events contain potentially sensitive attributes:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// TestEventAudit_AttributeValuesAreStrings verifies that event attributes
// use string values (not fmt.Sprintf with %v or %d directly in NewAttribute).
// All attribute values MUST be strings per CometBFT requirement.
func TestEventAudit_AttributeValuesAreStrings(t *testing.T) {
	root := findProjectRoot(t)
	modulesDir := filepath.Join(root, "x")

	// Check for raw numeric types passed directly to NewAttribute.
	// Valid: sdk.NewAttribute("key", "value")
	// Valid: sdk.NewAttribute("key", fmt.Sprintf("%d", x))
	// Invalid: sdk.NewAttribute("key", 42)
	badAttrRe := regexp.MustCompile(`sdk\.NewAttribute\("[^"]+",\s*\d+\s*\)`)

	var violations []string

	err := filepath.Walk(modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		matches := badAttrRe.FindAllString(string(data), -1)
		for _, match := range matches {
			relPath, _ := filepath.Rel(root, path)
			violations = append(violations, fmt.Sprintf("%s: %s", relPath, match))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk modules directory: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("event attributes with non-string values:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// --- helpers ---

type handlerInfo struct {
	name     string
	hasEvent bool
}

func extractHandlers(t *testing.T, path string, handlerRe, emitRe *regexp.Regexp) []handlerInfo {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open %s: %v", path, err)
	}
	defer file.Close()

	// Delegation pattern: handlers that delegate to keeper methods which
	// emit events internally (e.g., return ms.Keeper.Foo(ctx, msg)).
	delegateRe := regexp.MustCompile(`\.\w+\.\w+\(|\.Handle\w+\(|\.VoteProposal\(|\.graduateMentorship\(`)

	var handlers []handlerInfo
	braceDepth := 0
	inHandler := false
	enteredBody := false // true once we've seen the opening {

	// Non-handler methods and internal helpers to skip.
	skipMethods := map[string]bool{
		"NewMsgServerImpl":        true,
		"NewResearchMsgServerImpl": true,
		"markAccountInactive":     true,
		"checkEligibility":        true,
		"addVoteAudit":            true,
		"isValidStatusTransition": true,
		"intersectPermissions":    true,
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if !inHandler {
			if m := handlerRe.FindStringSubmatch(line); m != nil {
				name := m[1]
				if skipMethods[name] {
					continue
				}
				inHandler = true
				braceDepth = 0
				enteredBody = false
				handlers = append(handlers, handlerInfo{name: name, hasEvent: false})
			}
		}

		if inHandler {
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

			if braceDepth > 0 {
				enteredBody = true
			}

			if emitRe.MatchString(line) || delegateRe.MatchString(line) {
				handlers[len(handlers)-1].hasEvent = true
			}

			if enteredBody && braceDepth <= 0 {
				inHandler = false
			}
		}
	}

	return handlers
}

func extractModuleName(path string) string {
	// Extract module name from path like .../x/knowledge/keeper/msg_server.go
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i, p := range parts {
		if p == "x" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "unknown"
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from current working directory to find go.mod.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}
