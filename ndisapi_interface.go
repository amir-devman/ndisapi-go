//go:build windows

package ndisapi

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// NdisApiInterface defines the interface for NDISAPI driver interactions.
type NdisApiInterface interface {
	DeviceIoControl(service uint32, in unsafe.Pointer, sizeIn uint32, out unsafe.Pointer, sizeOut uint32, SizeRet *uint32, overlapped *windows.Overlapped) error
	IsDriverLoaded() bool
	Close()
	GetVersion() (uint32, error)
	GetIntermediateBufferPoolSize(size uint32) error
	GetTcpipBoundAdaptersInfo() (*TcpAdapterList, error)
	SetAdapterMode(currentMode *AdapterMode) error
	GetAdapterMode(currentMode *AdapterMode) error
	FlushAdapterPacketQueue(adapter Handle) error
	GetAdapterPacketQueueSize(adapter Handle, size *uint32) error
	SetPacketEvent(adapter Handle, win32Event windows.Handle) error
	ConvertWindows2000AdapterName(adapterName string) string
	InitializeFastIo(pFastIo *InitializeFastIOSection, dwSize uint32) bool
	AddSecondaryFastIo(fastIo *InitializeFastIOSection, size uint32) bool
	ReadPacketsUnsorted(packets []*IntermediateBuffer, dwPacketsNum uint32, pdwPacketsSuccess *uint32) bool
	SendPacketsToAdaptersUnsorted(packets []*IntermediateBuffer, dwPacketsNum uint32, pdwPacketSuccess *uint32) bool
	SendPacketsToMstcpUnsorted(packets []*IntermediateBuffer, dwPacketsNum uint32, pdwPacketSuccess *uint32) bool
	SendPacketToMstcp(packet *EtherRequest) error
	SendPacketToAdapter(packet *EtherRequest) error
	ReadPacket(packet *EtherRequest) bool
	SendPacketsToMstcp(packet *EtherMultiRequest) error
	SendPacketsToAdapter(packet *EtherMultiRequest) error
	ReadPackets(packet *EtherMultiRequest) bool
	SetPacketFilterTable(packet *StaticFilterTable) error
	AddStaticFilterFront(filter *StaticFilterEntry) error
	AddStaticFilterBack(filter *StaticFilterEntry) error
	InsertStaticFilter(filter *StaticFilterEntry, position uint32) error
	RemoveStaticFilter(filterID uint32) error
	ResetPacketFilterTable() error
	GetPacketFilterTableSize() (*uint32, error)
	GetPacketFilterTable() (*StaticFilterTable, error)
	GetPacketFilterTableResetStats() (*StaticFilterTable, error)
	SetPacketFilterCacheState(state bool) error
	SetPacketFragmentCacheState(state bool) error
	EnablePacketFilterCache() error
	DisablePacketFilterCache() error
	EnablePacketFragmentCache() error
	DisablePacketFragmentCache() error
	IsNdiswanInterfaces(adapterName, ndiswanName string) bool
	IsNdiswanIP(adapterName string) bool
	IsNdiswanIPv6(adapterName string) bool
	IsNdiswanBh(adapterName string) bool
	IsWindows10OrGreater() bool
}
