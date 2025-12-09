package dumper

import (
	"bytes"
	"compress/bzip2"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
	pb "github.com/OhMyDitzzy/go-payload-dumper/protos"
)

func processOperationType(op *pb.InstallOperation, data []byte, outFile, oldFile *os.File, blockSize uint64) error {
	opType := *op.Type

	switch opType {
	case pb.InstallOperation_REPLACE:
		return processReplace(op, data, outFile, blockSize)
	case pb.InstallOperation_REPLACE_BZ:
		return processReplaceBZ(op, data, outFile, blockSize)
	case pb.InstallOperation_REPLACE_XZ:
		return processReplaceXZ(op, data, outFile, blockSize)
	case pb.InstallOperation_ZSTD:
		return processZSTD(op, data, outFile, blockSize)
	case pb.InstallOperation_SOURCE_COPY:
		return processSourceCopy(op, outFile, oldFile, blockSize)
	case pb.InstallOperation_SOURCE_BSDIFF, pb.InstallOperation_BROTLI_BSDIFF:
		return processBSDIFF(op, data, outFile, oldFile, blockSize)
	case pb.InstallOperation_ZERO:
		return processZero(op, outFile, blockSize)
	default:
		return fmt.Errorf("unsupported operation type: %v", opType)
	}
}

func processReplace(op *pb.InstallOperation, data []byte, outFile *os.File, blockSize uint64) error {
	if len(op.DstExtents) == 0 {
		return fmt.Errorf("no destination extents")
	}
	offset := int64(*op.DstExtents[0].StartBlock * blockSize)
	_, err := outFile.Seek(offset, io.SeekStart)
	if err != nil {
		return err
	}
	_, err = outFile.Write(data)
	return err
}

func processReplaceBZ(op *pb.InstallOperation, data []byte, outFile *os.File, blockSize uint64) error {
	reader := bzip2.NewReader(bytes.NewReader(data))
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	if len(op.DstExtents) == 0 {
		return fmt.Errorf("no destination extents")
	}
	offset := int64(*op.DstExtents[0].StartBlock * blockSize)
	_, err = outFile.Seek(offset, io.SeekStart)
	if err != nil {
		return err
	}
	_, err = outFile.Write(decompressed)
	return err
}

func processReplaceXZ(op *pb.InstallOperation, data []byte, outFile *os.File, blockSize uint64) error {
	decompressed, err := decompressXZNative(data)
	if err != nil {
		decompressed, err = decompressXZCommand(data)
		if err != nil {
			return fmt.Errorf("xz decompression failed (native and command): %w", err)
		}
	}

	if len(op.DstExtents) == 0 {
		return fmt.Errorf("no destination extents")
	}
	offset := int64(*op.DstExtents[0].StartBlock * blockSize)
	_, err = outFile.Seek(offset, io.SeekStart)
	if err != nil {
		return err
	}
	_, err = outFile.Write(decompressed)
	return err
}

func decompressXZNative(data []byte) ([]byte, error) {
	reader, err := xz.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(reader)
}

func decompressXZCommand(data []byte) ([]byte, error) {
	if _, err := exec.LookPath("xz"); err != nil {
		return nil, fmt.Errorf("xz command not found in PATH")
	}

	cmd := exec.Command("xz", "-d", "-c")
	cmd.Stdin = bytes.NewReader(data)
	
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("xz command failed: %w, stderr: %s", err, errBuf.String())
	}

	return out.Bytes(), nil
}

func processZSTD(op *pb.InstallOperation, data []byte, outFile *os.File, blockSize uint64) error {
	decoder, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer decoder.Close()

	decompressed, err := io.ReadAll(decoder)
	if err != nil {
		return err
	}

	if len(op.DstExtents) == 0 {
		return fmt.Errorf("no destination extents")
	}
	offset := int64(*op.DstExtents[0].StartBlock * blockSize)
	_, err = outFile.Seek(offset, io.SeekStart)
	if err != nil {
		return err
	}
	_, err = outFile.Write(decompressed)
	return err
}

func processSourceCopy(op *pb.InstallOperation, outFile, oldFile *os.File, blockSize uint64) error {
	if oldFile == nil {
		return fmt.Errorf("SOURCE_COPY requires old file for differential OTA")
	}

	if len(op.DstExtents) == 0 {
		return fmt.Errorf("no destination extents")
	}
	outOffset := int64(*op.DstExtents[0].StartBlock * blockSize)
	_, err := outFile.Seek(outOffset, io.SeekStart)
	if err != nil {
		return err
	}

	for _, ext := range op.SrcExtents {
		offset := int64(*ext.StartBlock * blockSize)
		size := int64(*ext.NumBlocks * blockSize)

		_, err := oldFile.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}

		data := make([]byte, size)
		_, err = io.ReadFull(oldFile, data)
		if err != nil {
			return err
		}

		_, err = outFile.Write(data)
		if err != nil {
			return err
		}
	}

	return nil
}

func processZero(op *pb.InstallOperation, outFile *os.File, blockSize uint64) error {
	for _, ext := range op.DstExtents {
		offset := int64(*ext.StartBlock * blockSize)
		size := int64(*ext.NumBlocks * blockSize)

		_, err := outFile.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}

		zeros := make([]byte, size)
		_, err = outFile.Write(zeros)
		if err != nil {
			return err
		}
	}
	return nil
}

func processBSDIFF(op *pb.InstallOperation, data []byte, outFile, oldFile *os.File, blockSize uint64) error {
	if oldFile == nil {
		return fmt.Errorf("BSDIFF requires old file for differential OTA")
	}

	var oldData bytes.Buffer
	for _, ext := range op.SrcExtents {
		offset := int64(*ext.StartBlock * blockSize)
		size := int64(*ext.NumBlocks * blockSize)

		_, err := oldFile.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}

		buffer := make([]byte, size)
		_, err = io.ReadFull(oldFile, buffer)
		if err != nil {
			return err
		}

		oldData.Write(buffer)
	}

	patched, err := ApplyBSDIFF(oldData.Bytes(), data)
	if err != nil {
		return err
	}

	n := uint64(0)
	for _, ext := range op.DstExtents {
		offset := int64(*ext.StartBlock * blockSize)
		size := *ext.NumBlocks * blockSize

		_, err := outFile.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}

		end := n + size
		if end > uint64(len(patched)) {
			end = uint64(len(patched))
		}

		_, err = outFile.Write(patched[n:end])
		if err != nil {
			return err
		}

		n = end
	}

	return nil
}