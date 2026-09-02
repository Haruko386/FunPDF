//go:build windows

package service

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

const folderPickerMaxPath = 260

type browseInfo struct {
	hwndOwner      uintptr
	pidlRoot       uintptr
	pszDisplayName uintptr
	lpszTitle      uintptr
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

// pickDirectory opens the native folder browser dialog (SHBrowseForFolder) and
// returns the selected dir; empty string when the user cancels.
func pickDirectory() (string, error) {
	ole32 := syscall.NewLazyDLL("ole32.dll")
	shell32 := syscall.NewLazyDLL("shell32.dll")
	coInitializeEx := ole32.NewProc("CoInitializeEx")
	coUninitialize := ole32.NewProc("CoUninitialize")
	shBrowseForFolder := shell32.NewProc("SHBrowseForFolderW")
	shGetPathFromIDList := shell32.NewProc("SHGetPathFromIDListW")
	coTaskMemFree := ole32.NewProc("CoTaskMemFree")

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// COINIT_APARTMENTTHREADED so the dialog can run its own message loop.
	if hr, _, _ := coInitializeEx.Call(0, 2); hr == 0 || hr == 1 {
		defer coUninitialize.Call()
	}

	title, err := syscall.UTF16PtrFromString("选择缓存文件存储目录")
	if err != nil {
		return "", err
	}
	var displayName [folderPickerMaxPath]uint16

	// BIF_RETURNONLYFSDIRS | BIF_NEWDIALOGSTYLE
	const flags = 0x0001 | 0x0040
	bi := browseInfo{
		pszDisplayName: uintptr(unsafe.Pointer(&displayName[0])),
		lpszTitle:      uintptr(unsafe.Pointer(title)),
		ulFlags:        flags,
	}
	pidl, _, _ := shBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return "", nil
	}
	defer coTaskMemFree.Call(pidl)

	var pathBuf [folderPickerMaxPath]uint16
	if ret, _, _ := shGetPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&pathBuf[0]))); ret == 0 {
		return "", fmt.Errorf("resolve selected folder failed")
	}
	return syscall.UTF16ToString(pathBuf[:]), nil
}
