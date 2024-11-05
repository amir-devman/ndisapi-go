# NDISAPI-Go

NDISAPI-Go is a comprehensive user-mode interface library designed for seamless interaction with the [Windows Packet Filter](https://www.ntkernel.com/windows-packet-filter/) driver. It stands out by offering a straightforward, safe, and efficient interface for filtering (inspecting and modifying) raw network packets at the NDIS level of the network stack, ensuring minimal impact on network performance.

Windows Packet Filter (WinpkFilter) is a robust and efficient packet filtering framework tailored for Windows environments. It empowers developers to handle raw network packets at the NDIS level with ease, providing capabilities for packet inspection, modification, and control. WinpkFilter boasts user-friendly APIs, compatibility across various Windows versions, and streamlines network packet manipulation without the need for kernel-mode programming skills.

## Key Features

- **Network Adapter Management**: Enumerate and manage network adapter properties.
- **Packet Analysis and Modification**: Capture, filter, and modify network packets.
- **Packet Transmission**: Send raw packets directly through the network stack.
