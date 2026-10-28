package nas

import "golang.org/x/sys/windows"

func statFilesystem(path string) (uint64, uint64, error) {
	encodedPath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var availableBytes, totalBytes, freeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(
		encodedPath, &availableBytes, &totalBytes, &freeBytes,
	); err != nil {
		return 0, 0, err
	}
	return totalBytes, availableBytes, nil
}
