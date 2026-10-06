package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// holdTree puts this process in a job that ends every process in it when its
// last handle closes, which this process holds alone: the sh it starts and
// whatever that starts are in the job too, so a script-exe killed (a fake gh
// past itos's bound, bug 46) takes its script's processes with it, as killing
// the script itself does elsewhere. Windows starts children outside their
// parent's job only when asked to. A job that cannot be made is no error:
// the script still runs, its processes left to end on their own.
func holdTree() {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return
	}
	if err := windows.AssignProcessToJobObject(job, windows.CurrentProcess()); err != nil {
		windows.CloseHandle(job)
	}
}
