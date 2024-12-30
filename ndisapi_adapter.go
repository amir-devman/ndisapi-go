//go:build windows

package ndisapi

import (
	"bytes"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// TcpAdapterList structure used for requesting information about currently bound TCPIP adapters
type TcpAdapterList struct {
	AdapterCount      uint32                                     // Number of adapters
	AdapterNameList   [ADAPTER_LIST_SIZE][ADAPTER_NAME_SIZE]byte // Array of adapter names
	AdapterHandle     [ADAPTER_LIST_SIZE]Handle                  // Array of adapter handles, these are key handles for any adapter relative operation
	AdapterMediumList [ADAPTER_LIST_SIZE]uint32                  // List of adapter mediums
	CurrentAddress    [ADAPTER_LIST_SIZE][ETHER_ADDR_LENGTH]byte // current (configured) ethernet address
	MTU               [ADAPTER_LIST_SIZE]uint16                  // current adapter MTU
}

// AdapterMode used for setting adapter mode
type AdapterMode struct {
	AdapterHandle Handle
	Flags         uint32
}

// AdapterEvent used for setting up the event which driver sets once having packet in the queue for the processing
type AdapterEvent struct {
	AdapterHandle Handle
	Event         windows.Handle
}

// PacketOidData used for passing NDIS_REQUEST to driver
type PacketOidData struct {
	AdapterHandle Handle
	Oid           uint32
	Length        uint32
	Data          [AnySize]byte
}

// GetTcpipBoundAdaptersInfo retrieves the list of TCPIP-bound adapters.
func (a *NdisApi) GetTcpipBoundAdaptersInfo() (*TcpAdapterList, error) {
	var tcpAdapterList TcpAdapterList

	err := a.DeviceIoControl(
		IOCTL_NDISRD_GET_TCPIP_INTERFACES,
		unsafe.Pointer(&tcpAdapterList),
		uint32(unsafe.Sizeof(tcpAdapterList)),
		unsafe.Pointer(&tcpAdapterList),
		uint32(unsafe.Sizeof(tcpAdapterList)),
		nil,
		nil,
	)

	if err != nil {
		return nil, err
	}

	return &tcpAdapterList, nil
}

// SetAdapterMode sets the filter mode of the network adapter.
func (a *NdisApi) SetAdapterMode(currentMode *AdapterMode) error {
	return a.DeviceIoControl(
		IOCTL_NDISRD_SET_ADAPTER_MODE,
		unsafe.Pointer(currentMode),
		uint32(unsafe.Sizeof(AdapterMode{})),
		nil,
		0,
		nil, // Bytes Returned
		nil,
	)
}

// GetAdapterMode retrieves the filter mode of the network adapter.
func (a *NdisApi) GetAdapterMode(currentMode *AdapterMode) error {
	return a.DeviceIoControl(
		IOCTL_NDISRD_SET_ADAPTER_MODE,
		unsafe.Pointer(currentMode),
		uint32(unsafe.Sizeof(AdapterMode{})),
		unsafe.Pointer(currentMode),
		uint32(unsafe.Sizeof(AdapterMode{})),
		nil, // Bytes Returned
		nil,
	)
}

// ConvertWindows2000AdapterName converts an adapter's internal name to a user-friendly name on Windows 2000 and later.
func (a *NdisApi) ConvertWindows2000AdapterName(adapterName string) string {
	if a.IsNdiswanIP(adapterName) {
		return USER_NDISWANIP
	}
	if a.IsNdiswanBh(adapterName) {
		return USER_NDISWANBH
	}
	if a.IsNdiswanIPv6(adapterName) {
		return USER_NDISWANIPV6
	}

	adapterNameBytes := []byte((strings.TrimPrefix(adapterName, `\DEVICE\`)))
	adapterNameBytes = bytes.Trim(adapterNameBytes, "\x00")

	keyPath := REGSTR_NETWORK_CONTROL_KEY + string(adapterNameBytes) + REGSTR_VAL_CONNECTION

	key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.READ)
	if err != nil {
		return string(adapterNameBytes)
	}
	defer key.Close()

	val, _, err := key.GetStringValue(REGSTR_VAL_NAME)
	if err != nil {
		return string(adapterNameBytes)
	}

	return val
}