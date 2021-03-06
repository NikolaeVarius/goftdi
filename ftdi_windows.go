package ftdi

import "unsafe"
import "syscall"
import "bytes"
import "time"

func bytesToString(b []byte) string {
	n := bytes.Index(b, []byte{0})
	return string(b[:n])
}

var d2xx = syscall.MustLoadDLL("ftd2xx.dll")

var (
	createDeviceInfoList = d2xx.MustFindProc("FT_CreateDeviceInfoList")
	getDeviceInfoDetail  = d2xx.MustFindProc("FT_GetDeviceInfoDetail")
	ft_open              = d2xx.MustFindProc("FT_Open")
	ft_close             = d2xx.MustFindProc("FT_Close")
	ft_read              = d2xx.MustFindProc("FT_Read")
	ft_write             = d2xx.MustFindProc("FT_Write")
	ft_getStatus         = d2xx.MustFindProc("FT_GetStatus")
	ft_purge             = d2xx.MustFindProc("FT_Purge")
	setBaudRate          = d2xx.MustFindProc("FT_SetBaudRate")
	setBitMode           = d2xx.MustFindProc("FT_SetBitMode")
	setFlowControl       = d2xx.MustFindProc("FT_SetFlowControl")
	setLatency           = d2xx.MustFindProc("FT_SetLatencyTimer")
	setChars             = d2xx.MustFindProc("FT_SetChars")
	setLineProperty      = d2xx.MustFindProc("FT_SetDataCharacteristics")
	setTimeout           = d2xx.MustFindProc("FT_SetTimeouts")
	setTransferSize      = d2xx.MustFindProc("FT_SetUSBParameters")
	resetPort            = d2xx.MustFindProc("FT_ResetPort")
	resetDevice          = d2xx.MustFindProc("FT_ResetDevice")
)

type Device uintptr

type DeviceInfo struct {
	index         uint64
	flags         uint64
	dtype         uint64
	id            uint64
	location      uint64
	SerialNumber string
	Description   string
	handle        uintptr
}

func GetDeviceList() (di []DeviceInfo, e error) {
	var n uint32
	r, _, err := createDeviceInfoList.Call(uintptr(unsafe.Pointer(&n)))
	if r != 0 {
		return di, err
	}

	di = make([]DeviceInfo, n)
	for i := uint32(0); i < n; i++ {
		var d DeviceInfo
		var sn [16]byte
		var description [64]byte
		d.index = uint64(i)
		r, _, e = getDeviceInfoDetail.Call(uintptr(i),
			uintptr(unsafe.Pointer(&(d.flags))),
			uintptr(unsafe.Pointer(&d.dtype)),
			uintptr(unsafe.Pointer(&d.id)),
			uintptr(unsafe.Pointer(&d.location)),
			uintptr(unsafe.Pointer(&sn)),
			uintptr(unsafe.Pointer(&description)),
			uintptr(unsafe.Pointer(&d.handle)))
		if r != 0 {
			return di, e
		}
		d.SerialNumber = bytesToString(sn[:])
		d.Description = bytesToString(description[:])

		di[i] = d
	}
	return di, nil
}

func Open(di DeviceInfo) (*Device, error) {
	var dev Device
	r, _, e := ft_open.Call(uintptr(di.index), uintptr(unsafe.Pointer(&dev)))
	if r == 0 {
		return &dev, nil
	}
	return nil, e
}

func (d *Device) Close() (e error) {
	r, _, e := ft_close.Call(uintptr(*d))
	if r == 0 {
		return nil
	}
	return e
}

// Does this have Posix Counterpart?
func (d *Device) GetStatus() (rx_queue, tx_queue, events int32, e error) {
	r, _, e := ft_getStatus.Call(uintptr(*d),
		uintptr(unsafe.Pointer(&rx_queue)),
		uintptr(unsafe.Pointer(&tx_queue)),
		uintptr(unsafe.Pointer(&events)))
	if r == 0 {
		return rx_queue, tx_queue, events, nil
	}
	return rx_queue, tx_queue, events, e
}

//TODO: Need EOF logic for a closed device
func (d *Device) Read(p []byte) (n int, e error) {
	var bytesRead uint32
	bytesToRead := uint32(len(p))

    // Bugfix: Only read what's available immediately. On windows, you can't
    // trust ft_read to block until the requested amount of data is available. 
    // We also insert a delay to force the read to block for more data...
    // ugh... crappy FTDI drivers.
    rx_cnt, _, _, e := d.GetStatus()
    if bytesToRead > uint32(rx_cnt) {
        time.Sleep(20*time.Millisecond)
        bytesToRead = uint32(rx_cnt)
    }

	ptr := &p[0] //A reference to the first element of the underlying "array"
	r, _, e := ft_read.Call(uintptr(*d),
		uintptr(unsafe.Pointer(ptr)),
		uintptr(bytesToRead),
		uintptr(unsafe.Pointer(&bytesRead)))
	if r == 0 {
		return int(bytesRead), nil
	}
	return int(bytesRead), e
}

func (d *Device) Write(p []byte) (n int, e error) {
	var bytesWritten uint32
	bytesToWrite := uint32(len(p))
	ptr := &p[0] //A reference to the first element of the underlying "array"
	r, _, e := ft_write.Call(uintptr(*d),
		uintptr(unsafe.Pointer(ptr)),
		uintptr(bytesToWrite),
		uintptr(unsafe.Pointer(&bytesWritten)))
	if r == 0 {
		return int(bytesWritten), nil
	}
	return int(bytesWritten), e
}

func (d *Device) SetBaudRate(baud uint) (e error) {
	r, _, e := setBaudRate.Call(uintptr(*d), uintptr(uint32(baud)))
	if r == 0 {
		return nil
	}
	return e
}

// Set the 'event' and 'error' characheters. Disabled if the charachter is '0x00'.
func (d *Device) SetChars(event, err byte) (e error) {
	r, _, e := setChars.Call(uintptr(*d),
		uintptr(event),
		uintptr(event),
		uintptr(err),
		uintptr(err))
	if r == 0 {
		return nil
	}
	return e
}

func (d *Device) SetBitMode(mode BitMode) (e error) {
	r, _, e := setBitMode.Call(uintptr(*d),
		uintptr(0x00), // All pins set to input
		uintptr(byte(mode)))
	if r == 0 {
		return nil
	}
	return e
}

func (d *Device) SetFlowControl(f FlowControl) (e error) {
	r, _, e := setFlowControl.Call(uintptr(*d),
		uintptr(uint16(f)), // All pins set to input
		uintptr(0x11),      // XON Character
		uintptr(0x13))      // XOFF Character
	if r == 0 {
		return nil
	}
	return e
}

// Set latency in milliseconds. Valid between 2 and 255.
func (d *Device) SetLatency(latency int) (e error) {
	r, _, e := setLatency.Call(uintptr(*d), uintptr(byte(latency)))
	if r == 0 {
		return nil
	}
	return e
}

// Set the transfer size. Valid between 64 and 64k bytes in 64-byte increments.
func (d *Device) SetTransferSize(read_size, write_size int) (e error) {
	r, _, e := setTransferSize.Call(uintptr(*d),
		uintptr(uint32(read_size)),
		uintptr(uint32(write_size)))
	if r == 0 {
		return nil
	}
	return e
}

func (d *Device) SetLineProperty(props LineProperties) (e error) {
	r, _, e := setLineProperty.Call(uintptr(*d),
		uintptr(byte(props.Bits)),
		uintptr(byte(props.StopBits)),
		uintptr(byte(props.Parity)))
	if r == 0 {
		return nil
	}
	return e
}

func (d *Device) SetTimeout(read_timeout, write_timeout int) (e error) {
	r, _, e := setTimeout.Call(uintptr(*d),
		uintptr(uint32(read_timeout)),
		uintptr(uint32(write_timeout)))
	if r == 0 {
		return nil
	}
	return e
}

func (d *Device) Reset() (e error) {
	r, _, e := resetDevice.Call(uintptr(*d))
	if r == 0 {
		return nil
	}
	return e
}

func (d *Device) Purge() (e error) {
	// Purge both RX and TX buffers
	r, _, e := ft_purge.Call(uintptr(*d), uintptr(0x01|0x02))
	if r == 0 {
		return nil
	}
	return e
}
