//go:build windows

package store

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func assertPrivatePath(t *testing.T, path string, _ bool) {
	t.Helper()
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("private path DACL inherits access entries")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if dacl == nil {
		t.Fatal("private path DACL is absent")
	}
	if dacl.AceCount != 1 {
		t.Fatalf("private path ACE count=%d want=1", dacl.AceCount)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	const fileAllAccess = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | windows.ACCESS_MASK(0x1ff)
	if !aceSID.Equals(user.User.Sid) || ace.Mask&fileAllAccess != fileAllAccess {
		t.Fatalf("private path ACE is not current-user full control: %s", descriptor.String())
	}
}
