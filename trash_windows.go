//go:build windows

package main

import (
	"os/exec"
	"syscall"
	"unsafe"
)

// SHFileOperation flags
const (
	FO_DELETE          = 0x0003
	FOF_ALLOWUNDO      = 0x0040 // mover a papelera en vez de borrar
	FOF_NOCONFIRMATION = 0x0010 // sin diálogo por archivo
	FOF_SILENT         = 0x0004 // sin barra de progreso de Windows
)

type SHFILEOPSTRUCT struct {
	Hwnd                  uintptr
	WFunc                 uint32
	PFrom                 uintptr
	PTo                   uintptr
	FFlags                uint16
	FAnyOperationsAborted int32
	HNameMappings         uintptr
	LpszProgressTitle     uintptr
}

var shell32 = syscall.NewLazyDLL("shell32.dll")
var shFileOperationW = shell32.NewProc("SHFileOperationW")

// moveToTrash mueve todos los archivos a la papelera en UNA sola
// operación nativa de Windows. El usuario ve un único diálogo de
// confirmación con todos los archivos agrupados.
func moveToTrash(paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	// SHFileOperationW espera un string multi-nulo:
	// "ruta1\0ruta2\0ruta3\0\0" (terminado con doble null)
	var buf []uint16
	for _, p := range paths {
		encoded, err := syscall.UTF16FromString(p)
		if err != nil {
			continue
		}
		// UTF16FromString ya añade un null al final; lo incluimos
		buf = append(buf, encoded...)
	}
	// Segundo null final para cerrar la lista
	buf = append(buf, 0)

	op := SHFILEOPSTRUCT{
		WFunc:  FO_DELETE,
		PFrom:  uintptr(unsafe.Pointer(&buf[0])),
		FFlags: FOF_ALLOWUNDO | FOF_NOCONFIRMATION,
	}

	ret, _, _ := shFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return syscall.Errno(ret)
	}
	return nil
}

// openWithDefault abre el archivo con la aplicación predeterminada de Windows
func openWithDefault(path string) error {
	cmd := exec.Command("cmd", "/c", "start", "", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
