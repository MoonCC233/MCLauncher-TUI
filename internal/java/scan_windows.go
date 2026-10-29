//go:build windows

package java

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// addWindowsExtraCandidates goes beyond the environment variables and the
// system drive. Java is often installed on another local disk, and portable
// JDKs are not registered anywhere, so inspect the registry plus the common
// install locations on every fixed drive.
func addWindowsExtraCandidates(out *[]candidate) {
	for _, home := range windowsRegistryJavaHomes() {
		addPath(out, home, "Registry")
	}
	for _, root := range windowsFixedDrives() {
		addWindowsDriveCandidates(root, out)
	}
}

func windowsFixedDrives() []string {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var roots []string
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		path, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if windows.GetDriveType(path) != windows.DRIVE_FIXED {
			continue
		}
		roots = append(roots, root)
	}
	return roots
}

func windowsRegistryJavaHomes() []string {
	roots := []string{
		`SOFTWARE\JavaSoft\Java Development Kit`,
		`SOFTWARE\JavaSoft\Java Runtime Environment`,
		`SOFTWARE\JavaSoft\JDK`,
		`SOFTWARE\JavaSoft\JRE`,
		`SOFTWARE\Eclipse Adoptium\JDK`,
		`SOFTWARE\Eclipse Foundation\JDK`,
		`SOFTWARE\AdoptOpenJDK\JDK`,
		`SOFTWARE\Microsoft\JDK`,
		`SOFTWARE\Amazon\Corretto`,
		`SOFTWARE\Azul Systems\Zulu`,
		`SOFTWARE\BellSoft\Liberica`,
		`SOFTWARE\Semeru`,
		`SOFTWARE\IBM\Java`,
	}
	all := make([]string, 0, len(roots)*2)
	for _, root := range roots {
		all = append(all, root, `SOFTWARE\WOW6432Node\`+root)
	}
	var homes []string
	for _, hive := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		for _, root := range all {
			collectRegistryJavaHomes(hive, root, 2, &homes)
		}
	}
	return homes
}

func collectRegistryJavaHomes(hive registry.Key, path string, depth int, homes *[]string) {
	key, err := registry.OpenKey(hive, path, registry.READ)
	if err != nil {
		return
	}
	defer key.Close()
	for _, valueName := range []string{"JavaHome", "JavaHome64", "Path", "InstallationPath", "InstallDir", "Home"} {
		if value, _, err := key.GetStringValue(valueName); err == nil && strings.TrimSpace(value) != "" {
			*homes = append(*homes, value)
		}
	}
	if depth <= 0 {
		return
	}
	names, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return
	}
	for _, name := range names {
		collectRegistryJavaHomes(hive, path+`\`+name, depth-1, homes)
	}
}

func addWindowsDriveCandidates(root string, out *[]candidate) {
	source := "Disk " + strings.TrimSuffix(root, `\`)
	globCandidates(windowsDrivePatterns(root), source, out)
	// These roots are usually small; walk them to catch nested vendor folders.
	for _, dir := range []string{"Java", "JDK"} {
		walkCandidates(filepath.Join(root, dir), 4, source, out)
	}
}

func windowsDrivePatterns(root string) []string {
	patterns := []string{
		filepath.Join(root, "Java", "*", "bin", javaExe()),
		filepath.Join(root, "Java", "*", "*", "bin", javaExe()),
		filepath.Join(root, "jdk*", "bin", javaExe()),
		filepath.Join(root, "java*", "bin", javaExe()),
		filepath.Join(root, "graalvm*", "bin", javaExe()),
		filepath.Join(root, "zulu*", "bin", javaExe()),
		filepath.Join(root, "Program Files", "Java", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files (x86)", "Java", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files", "Eclipse Adoptium", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files (x86)", "Eclipse Adoptium", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files", "Eclipse Foundation", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files", "AdoptOpenJDK", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files", "Microsoft", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files (x86)", "Microsoft", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files", "Amazon Corretto", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files (x86)", "Amazon Corretto", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files", "Zulu", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files (x86)", "Zulu", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files", "BellSoft", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files (x86)", "BellSoft", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files", "Semeru", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files (x86)", "Semeru", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files", "*", "*", "bin", javaExe()),
		filepath.Join(root, "Program Files (x86)", "*", "*", "bin", javaExe()),
		// Bundled runtimes shipped with IDEs.
		filepath.Join(root, "Program Files", "JetBrains", "*", "jbr", "bin", javaExe()),
		filepath.Join(root, "Program Files", "JetBrains", "*", "*", "jbr", "bin", javaExe()),
		filepath.Join(root, "Program Files", "Android", "Android Studio", "jbr", "bin", javaExe()),
		filepath.Join(root, "Program Files", "Android", "Android Studio", "jre", "bin", javaExe()),
	}
	for _, dir := range []string{
		"Development", "Dev", "Tools", "Software", "Apps", "Environment", "env", "SDK", "SDKs", "JDK", "Java",
	} {
		patterns = append(patterns,
			filepath.Join(root, dir, "*", "bin", javaExe()),
			filepath.Join(root, dir, "*", "*", "bin", javaExe()),
		)
	}
	return patterns
}
