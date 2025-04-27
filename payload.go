package ctrl

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type Payload struct {
	Swap    []float32
	Fail    int32
	InFile  []byte
	OutName []byte
	Msg     []byte
	// GH-Cp gen: Added fields for file transfer
	FileContent    []byte // Content of the controller input file
	ServerFilePath []byte // Path where the file should be stored on server
	buffer         bytes.Buffer
}

func (p *Payload) MarshalBinary() ([]byte, error) {
	p.buffer.Reset()
	err := binary.Write(&p.buffer, binary.LittleEndian, uint32(len(p.Swap)))
	if err != nil {
		return nil, err
	}
	err = binary.Write(&p.buffer, binary.LittleEndian, uint32(len(p.InFile)))
	if err != nil {
		return nil, err
	}
	err = binary.Write(&p.buffer, binary.LittleEndian, uint32(len(p.OutName)))
	if err != nil {
		return nil, err
	}
	err = binary.Write(&p.buffer, binary.LittleEndian, uint32(len(p.Msg)))
	if err != nil {
		return nil, err
	}
	// GH-Cp gen: Added writing FileContent and ServerFilePath lengths
	err = binary.Write(&p.buffer, binary.LittleEndian, uint32(len(p.FileContent)))
	if err != nil {
		return nil, err
	}
	err = binary.Write(&p.buffer, binary.LittleEndian, uint32(len(p.ServerFilePath)))
	if err != nil {
		return nil, err
	}
	err = binary.Write(&p.buffer, binary.LittleEndian, p.Swap)
	if err != nil {
		return nil, err
	}
	err = binary.Write(&p.buffer, binary.LittleEndian, p.Fail)
	if err != nil {
		return nil, err
	}
	err = binary.Write(&p.buffer, binary.LittleEndian, p.InFile)
	if err != nil {
		return nil, err
	}
	err = binary.Write(&p.buffer, binary.LittleEndian, p.OutName)
	if err != nil {
		return nil, err
	}
	err = binary.Write(&p.buffer, binary.LittleEndian, p.Msg)
	if err != nil {
		return nil, err
	}
	// GH-Cp gen: Added writing FileContent and ServerFilePath data
	err = binary.Write(&p.buffer, binary.LittleEndian, p.FileContent)
	if err != nil {
		return nil, err
	}
	err = binary.Write(&p.buffer, binary.LittleEndian, p.ServerFilePath)
	if err != nil {
		return nil, err
	}
	return p.buffer.Bytes(), nil
}

func (p *Payload) UnmarshalBinary(data []byte) error {

	// Create a new bytes reader to read the data
	r := bytes.NewReader(data)

	// Read the lengths of the fields
	var swapLen, inFileLen, outNameLen, msgLen uint32
	// GH-Cp gen: Added variables for FileContent and ServerFilePath lengths
	var fileContentLen, serverFilePathLen uint32

	err := binary.Read(r, binary.LittleEndian, &swapLen)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &inFileLen)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &outNameLen)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &msgLen)
	if err != nil {
		return err
	}
	// GH-Cp gen: Read lengths of FileContent and ServerFilePath
	err = binary.Read(r, binary.LittleEndian, &fileContentLen)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &serverFilePathLen)
	if err != nil {
		return err
	}

	// Allocate slices of the appropriate size if they don't match
	if len(p.Swap) != int(swapLen) {
		p.Swap = make([]float32, swapLen)
	}
	if len(p.InFile) != int(inFileLen) {
		p.InFile = make([]byte, inFileLen)
	}
	if len(p.OutName) != int(outNameLen) {
		p.OutName = make([]byte, outNameLen)
	}
	if len(p.Msg) != int(msgLen) {
		p.Msg = make([]byte, msgLen)
	}
	// GH-Cp gen: Allocate FileContent and ServerFilePath slices
	if len(p.FileContent) != int(fileContentLen) {
		p.FileContent = make([]byte, fileContentLen)
	}
	if len(p.ServerFilePath) != int(serverFilePathLen) {
		p.ServerFilePath = make([]byte, serverFilePathLen)
	}

	// Read the fields from the buffer
	err = binary.Read(r, binary.LittleEndian, &p.Swap)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &p.Fail)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &p.InFile)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &p.OutName)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &p.Msg)
	if err != nil {
		return err
	}
	// GH-Cp gen: Read FileContent and ServerFilePath data
	err = binary.Read(r, binary.LittleEndian, &p.FileContent)
	if err != nil {
		return err
	}
	err = binary.Read(r, binary.LittleEndian, &p.ServerFilePath)
	if err != nil {
		return err
	}
	return nil
}

func (p Payload) String() string {
	i0InFile := bytes.IndexByte(p.InFile, 0)
	if i0InFile < 0 {
		i0InFile = len(p.InFile)
	}
	i0OutName := bytes.IndexByte(p.OutName, 0)
	if i0OutName < 0 {
		i0OutName = len(p.OutName)
	}
	i0Msg := bytes.IndexByte(p.Msg, 0)
	if i0Msg < 0 {
		i0Msg = len(p.Msg)
	}
	// GH-Cp gen: Added handling for ServerFilePath
	i0ServerFilePath := bytes.IndexByte(p.ServerFilePath, 0)
	if i0ServerFilePath < 0 {
		i0ServerFilePath = len(p.ServerFilePath)
	}

	// GH-Cp gen: Modified string formatting to include new fields
	return fmt.Sprintf("avrSWAP: 	%v\n"+
		"aviFAIL: 	%v\n"+
		"accINFILE:  '%s'\n"+
		"avcOUTNAME: '%s'\n"+
		"avcMSG:     '%s'\n"+
		"ServerFilePath: '%s'\n"+
		"FileContent: [%d bytes]\n",
		p.Swap[:129],
		p.Fail,
		p.InFile[:i0InFile],
		p.OutName[:i0OutName],
		p.Msg[:i0Msg],
		p.ServerFilePath[:i0ServerFilePath],
		len(p.FileContent))
}
