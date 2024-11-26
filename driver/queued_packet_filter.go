//go:build windows

package driver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	A "github.com/amir-devman/ndisapi-go"
	N "github.com/amir-devman/ndisapi-go/netlib"

	"golang.org/x/sys/windows"
)

type QueuedPacketFilter struct {
	*A.NdisApi

	adapters *A.TcpAdapterList

	filterIncomingPacket func(handle A.Handle, buffer *A.IntermediateBuffer) A.FilterAction
	filterOutgoingPacket func(handle A.Handle, buffer *A.IntermediateBuffer) A.FilterAction
	filterState          N.FilterState
	networkInterfaces    []*NetworkAdapter
	adapter              int

	wg         sync.WaitGroup
	cancelFunc context.CancelFunc

	packetReadChan         chan *PacketBlock
	packetProcessChan      chan *PacketBlock
	packetWriteMstcpChan   chan *PacketBlock
	packetWriteAdapterChan chan *PacketBlock
}

func NewQueuedPacketFilter(api *A.NdisApi, adapters *A.TcpAdapterList, in, out func(handle A.Handle, buffer *A.IntermediateBuffer) A.FilterAction) (*QueuedPacketFilter, error) {
	filter := &QueuedPacketFilter{
		NdisApi:  api,
		adapters: adapters,

		filterIncomingPacket: in,
		filterOutgoingPacket: out,
		filterState:          N.Stopped,
		adapter:              0,

		packetReadChan:         make(chan *PacketBlock, A.MaximumPacketBlock),
		packetProcessChan:      make(chan *PacketBlock, A.MaximumPacketBlock),
		packetWriteMstcpChan:   make(chan *PacketBlock, A.MaximumPacketBlock),
		packetWriteAdapterChan: make(chan *PacketBlock, A.MaximumPacketBlock),
	}

	err := filter.initializeNetworkInterfaces()
	if err != nil {
		return nil, err
	}

	return filter, nil
}

func (f *QueuedPacketFilter) initFilter() error {
	for i := 0; i < A.MaximumPacketBlock; i++ {
		packetBlock := NewPacketBlock(f.networkInterfaces[f.adapter].GetAdapter())
		f.packetReadChan <- packetBlock
	}

	// Set events for helper driver
	if err := f.networkInterfaces[f.adapter].SetPacketEvent(); err != nil {
		for len(f.packetReadChan) > 0 {
			<-f.packetReadChan
		}
		return err
	}

	f.networkInterfaces[f.adapter].SetMode(
		func() uint32 {
			mode := uint32(0)
			if f.filterOutgoingPacket != nil {
				mode |= A.MSTCP_FLAG_SENT_TUNNEL
			}
			if f.filterIncomingPacket != nil {
				mode |= A.MSTCP_FLAG_RECV_TUNNEL
			}
			return mode
		}(),
	)

	return nil
}

func (f *QueuedPacketFilter) ReleaseFilter() {
	f.networkInterfaces[f.adapter].Release()

	// TODO: need more review

	// Clear all queues
	for len(f.packetReadChan) > 0 {
		<-f.packetReadChan
	}

	for len(f.packetProcessChan) > 0 {
		<-f.packetProcessChan
	}

	for len(f.packetWriteMstcpChan) > 0 {
		<-f.packetWriteMstcpChan
	}

	for len(f.packetWriteAdapterChan) > 0 {
		<-f.packetWriteAdapterChan
	}
}

func (f *QueuedPacketFilter) Reconfigure() error {
	if f.filterState != N.Stopped {
		return errors.New("filter is not stopped")
	}

	f.networkInterfaces = make([]*NetworkAdapter, 0)
	if err := f.initializeNetworkInterfaces(); err != nil {
		return err
	}

	return nil
}

func (f *QueuedPacketFilter) StartFilter(adapterIdx int) error {
	if f.filterState != N.Stopped {
		return errors.New("filter is not stopped")
	}

	f.filterState = N.Starting
	f.adapter = adapterIdx

	if err := f.initFilter(); err != nil {
		f.filterState = N.Stopped
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	f.cancelFunc = cancel

	f.wg.Add(4)

	go f.packetRead(ctx)
	go f.packetProcess(ctx)
	go f.packetWriteMstcp(ctx)
	go f.packetWriteAdapter(ctx)

	f.filterState = N.Running

	return nil
}

func (f *QueuedPacketFilter) StopFilter() error {
	if f.filterState != N.Running {
		return errors.New("filter is not running")
	}

	f.filterState = N.Stopping

	// Cancel the context to stop all goroutines
	if f.cancelFunc != nil {
		f.cancelFunc()
	}

	f.filterState = N.Stopped

	return nil
}

func (f *QueuedPacketFilter) GetInterfaceNamesList() []string {
	names := make([]string, len(f.networkInterfaces))
	for i, iface := range f.networkInterfaces {
		names[i] = iface.FriendlyName
	}
	return names
}

func (f *QueuedPacketFilter) initializeNetworkInterfaces() error {
	for i := range f.adapters.AdapterCount {
		name := string(f.adapters.AdapterNameList[i][:])
		adapterHandle := f.adapters.AdapterHandle[i]
		currentAddress := f.adapters.CurrentAddress[i]
		medium := f.adapters.AdapterMediumList[i]
		mtu := f.adapters.MTU[i]

		friendlyName := f.ConvertWindows2000AdapterName(name)

		networkAdapter, err := NewNetworkAdapter(f.NdisApi, adapterHandle, currentAddress, name, friendlyName, medium, mtu)
		if err != nil {
			fmt.Println("error creating network adapter", err.Error())
			continue
		}
		f.networkInterfaces = append(f.networkInterfaces, networkAdapter)
	}

	return nil
}

func (f *QueuedPacketFilter) InsertPacketToMstcp(packetData *A.IntermediateBuffer) error {
	request := &A.EtherRequest{
		AdapterHandle: f.networkInterfaces[f.adapter].GetAdapter(),
		EthernetPacket: A.EthernetPacket{
			Buffer: packetData,
		},
	}

	return f.SendPacketToMstcp(request)
}

func (f *QueuedPacketFilter) InsertPacketToAdapter(packetData *A.IntermediateBuffer) error {
	request := &A.EtherRequest{
		AdapterHandle: f.networkInterfaces[f.adapter].GetAdapter(),
		EthernetPacket: A.EthernetPacket{
			Buffer: packetData,
		},
	}

	return f.SendPacketToAdapter(request)
}

func (q *QueuedPacketFilter) packetRead(ctx context.Context) {
	defer q.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if q.filterState != N.Running {
				return
			}

			packetBlock := <-q.packetReadChan

			readRequest := packetBlock.GetReadRequest()

			for q.filterState == N.Running {
				q.networkInterfaces[q.adapter].WaitEvent(windows.INFINITE)
				q.networkInterfaces[q.adapter].ResetEvent()

				if !q.ReadPackets(readRequest) {
					break
				}
			}

			q.packetProcessChan <- packetBlock
		}
	}
}

func (q *QueuedPacketFilter) packetProcess(ctx context.Context) {
	defer q.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if q.filterState != N.Running {
				return
			}

			packetBlock := <-q.packetProcessChan

			readRequest := packetBlock.GetReadRequest()
			writeAdapterRequest := packetBlock.GetWriteAdapterRequest()
			writeMstcpRequest := packetBlock.GetWriteMstcpRequest()

			for i := 0; i < int(readRequest.PacketsSuccess); i++ {
				packetAction := A.FilterActionPass

				if packetBlock.packetBuffer[i].DeviceFlags == A.PACKET_FLAG_ON_SEND {
					if q.filterOutgoingPacket != nil {
						packetAction = q.filterOutgoingPacket(readRequest.AdapterHandle, &packetBlock.packetBuffer[i])
					}
				} else {
					if q.filterIncomingPacket != nil {
						packetAction = q.filterIncomingPacket(readRequest.AdapterHandle, &packetBlock.packetBuffer[i])
					}
				}

				if packetAction == A.FilterActionPass {
					if packetBlock.packetBuffer[i].DeviceFlags == A.PACKET_FLAG_ON_SEND {
						writeAdapterRequest.EthernetPackets[writeAdapterRequest.PacketsNumber].Buffer = &packetBlock.packetBuffer[i]
						writeAdapterRequest.PacketsNumber++
					} else {
						writeMstcpRequest.EthernetPackets[writeMstcpRequest.PacketsNumber].Buffer = &packetBlock.packetBuffer[i]
						writeMstcpRequest.PacketsNumber++
					}
				} else if packetAction == A.FilterActionRedirect {
					if packetBlock.packetBuffer[i].DeviceFlags == A.PACKET_FLAG_ON_RECEIVE {
						writeAdapterRequest.EthernetPackets[writeAdapterRequest.PacketsNumber].Buffer = &packetBlock.packetBuffer[i]
						writeAdapterRequest.PacketsNumber++
					} else {
						writeMstcpRequest.EthernetPackets[writeMstcpRequest.PacketsNumber].Buffer = &packetBlock.packetBuffer[i]
						writeMstcpRequest.PacketsNumber++
					}
				}
			}

			readRequest.PacketsSuccess = 0

			q.packetWriteMstcpChan <- packetBlock
		}
	}
}

func (q *QueuedPacketFilter) packetWriteMstcp(ctx context.Context) {
	defer q.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if q.filterState != N.Running {
				return
			}

			packetBlock := <-q.packetWriteMstcpChan

			writeMstcpRequest := packetBlock.GetWriteMstcpRequest()
			if writeMstcpRequest.PacketsNumber > 0 {
				q.SendPacketsToMstcp(writeMstcpRequest)
				writeMstcpRequest.PacketsNumber = 0
			}

			q.packetWriteAdapterChan <- packetBlock
		}
	}
}

func (q *QueuedPacketFilter) packetWriteAdapter(ctx context.Context) {
	defer q.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if q.filterState != N.Running {
				return
			}

			packetBlock := <-q.packetWriteAdapterChan

			writeAdapterRequest := packetBlock.GetWriteAdapterRequest()
			if writeAdapterRequest.PacketsNumber > 0 {
				q.SendPacketsToAdapter(writeAdapterRequest)
				writeAdapterRequest.PacketsNumber = 0
			}

			q.packetReadChan <- packetBlock
		}
	}
}

func (f *QueuedPacketFilter) GetInterfaceHWList() []string {
	names := make([]string, len(f.networkInterfaces))
	for i, iface := range f.networkInterfaces {
		log.Println(i, iface)
	}
	return names
}

func (f *QueuedPacketFilter) GetFilterState() N.FilterState {
	return f.filterState
}