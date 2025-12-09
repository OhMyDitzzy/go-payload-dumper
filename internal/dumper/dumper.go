package dumper

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	pb "github.com/OhMyDitzzy/go-payload-dumper/protos"
)

const (
	Magic        = "CrAU"
	FileFormatV2 = 2
)

type Dumper struct {
	payloadFile io.ReadSeeker
	closer      io.Closer
	manifest    *pb.DeltaArchiveManifest
	dataOffset  int64
	outDir      string
	oldDir      string
	useDiff     bool
}

func New(payloadPath, outDir, oldDir string, useDiff bool) (*Dumper, error) {
	file, closer, err := openPayloadFile(payloadPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open payload: %w", err)
	}

	d := &Dumper{
		payloadFile: file,
		closer:      closer,
		outDir:      outDir,
		oldDir:      oldDir,
		useDiff:     useDiff,
	}

	if err := d.parseHeader(); err != nil {
		closer.Close()
		return nil, fmt.Errorf("failed to parse header: %w", err)
	}

	return d, nil
}

func (d *Dumper) Close() error {
	if d.closer != nil {
		return d.closer.Close()
	}
	return nil
}

func openPayloadFile(path string) (io.ReadSeeker, io.Closer, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return openRemoteFile(path)
	}
	return openLocalFile(path)
}

func openLocalFile(path string) (io.ReadSeeker, io.Closer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		stat, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, nil, err
		}

		zr, err := zip.NewReader(f, stat.Size())
		if err != nil {
			f.Close()
			return nil, nil, err
		}

		for _, file := range zr.File {
			if file.Name == "payload.bin" {
				rc, err := file.Open()
				if err != nil {
					f.Close()
					return nil, nil, err
				}

				data, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					f.Close()
					return nil, nil, err
				}

				f.Close()
				reader := bytes.NewReader(data)
				return reader, io.NopCloser(reader), nil
			}
		}
		f.Close()
		return nil, nil, fmt.Errorf("payload.bin not found in zip")
	}

	return f, f, nil
}

func openRemoteFile(url string) (io.ReadSeeker, io.Closer, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, err
	}

	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, nil, err
	}

	if strings.HasSuffix(strings.ToLower(url), ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, nil, err
		}

		for _, file := range zr.File {
			if file.Name == "payload.bin" {
				rc, err := file.Open()
				if err != nil {
					return nil, nil, err
				}

				payloadData, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					return nil, nil, err
				}

				reader := bytes.NewReader(payloadData)
				return reader, io.NopCloser(reader), nil
			}
		}
		return nil, nil, fmt.Errorf("payload.bin not found in zip")
	}

	reader := bytes.NewReader(data)
	return reader, io.NopCloser(reader), nil
}

func (d *Dumper) parseHeader() error {
	magic := make([]byte, 4)
	if _, err := io.ReadFull(d.payloadFile, magic); err != nil {
		return err
	}
	if string(magic) != Magic {
		return fmt.Errorf("invalid magic header")
	}

	var fileFormatVersion uint64
	if err := binary.Read(d.payloadFile, binary.BigEndian, &fileFormatVersion); err != nil {
		return err
	}
	if fileFormatVersion != FileFormatV2 {
		return fmt.Errorf("unsupported file format version: %d", fileFormatVersion)
	}

	var manifestSize uint64
	if err := binary.Read(d.payloadFile, binary.BigEndian, &manifestSize); err != nil {
		return err
	}

	var metadataSignatureSize uint32
	if err := binary.Read(d.payloadFile, binary.BigEndian, &metadataSignatureSize); err != nil {
		return err
	}

	manifestData := make([]byte, manifestSize)
	if _, err := io.ReadFull(d.payloadFile, manifestData); err != nil {
		return err
	}

	if metadataSignatureSize > 0 {
		if _, err := d.payloadFile.Seek(int64(metadataSignatureSize), io.SeekCurrent); err != nil {
			return err
		}
	}

	d.dataOffset, _ = d.payloadFile.Seek(0, io.SeekCurrent)

	d.manifest = &pb.DeltaArchiveManifest{}
	if err := proto.Unmarshal(manifestData, d.manifest); err != nil {
		return err
	}

	return nil
}

func (d *Dumper) Extract(images []string) error {
	blockSize := uint64(4096)
	if d.manifest.BlockSize != nil {
		blockSize = uint64(*d.manifest.BlockSize)
	}

	partitions := d.manifest.Partitions
	if len(images) > 0 {
		partitions = d.filterPartitions(images)
		if len(partitions) == 0 {
			return fmt.Errorf("no matching partitions found")
		}
	}

	for i, part := range partitions {
		if err := d.dumpPartition(part, blockSize, i+1, len(partitions)); err != nil {
			return fmt.Errorf("failed to dump partition %s: %w", *part.PartitionName, err)
		}
	}

	return nil
}

func (d *Dumper) filterPartitions(images []string) []*pb.PartitionUpdate {
	var result []*pb.PartitionUpdate
	imageMap := make(map[string]bool)
	for _, img := range images {
		imageMap[strings.TrimSpace(img)] = true
	}

	for _, part := range d.manifest.Partitions {
		if imageMap[*part.PartitionName] {
			result = append(result, part)
		}
	}

	return result
}

func (d *Dumper) dumpPartition(part *pb.PartitionUpdate, blockSize uint64, current, total int) error {
	partName := *part.PartitionName
	totalOps := len(part.Operations)

	outPath := filepath.Join(d.outDir, partName+".img")
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	var oldFile *os.File
	if d.useDiff {
		oldPath := filepath.Join(d.oldDir, partName+".img")
		oldFile, _ = os.Open(oldPath)
		if oldFile != nil {
			defer oldFile.Close()
		}
	}

	var totalSize uint64
	if part.NewPartitionInfo != nil && part.NewPartitionInfo.Size != nil {
		totalSize = *part.NewPartitionInfo.Size
	} else {
		for _, op := range part.Operations {
			for _, extent := range op.DstExtents {
				if extent.NumBlocks != nil {
					totalSize += *extent.NumBlocks * blockSize
				}
			}
		}
	}

	startTime := time.Now()
	var processedSize uint64

	fmt.Printf("Processing '%s' partitions [%s]   0%% | 0B/%s | Elapsed: 00:00:00 | ETA: --:--:--\r", 
		partName, strings.Repeat("-", 30), formatBytes(totalSize))

	for i, op := range part.Operations {
		if err := d.processOperation(op, outFile, oldFile, blockSize); err != nil {
			return err
		}

		for _, extent := range op.DstExtents {
			if extent.NumBlocks != nil {
				processedSize += *extent.NumBlocks * blockSize
			}
		}

		progress := float64(i+1) / float64(totalOps) * 100
		barLength := 30
		filled := int(float64(barLength) * float64(i+1) / float64(totalOps))

		var bar string
		if filled > 0 {
			bar = strings.Repeat("=", filled-1) + ">" + strings.Repeat("-", barLength-filled)
		} else {
			bar = strings.Repeat("-", barLength)
		}

		elapsed := time.Since(startTime)
		elapsedStr := formatDuration(elapsed)

		var etaStr string
		if i > 0 {
			avgTimePerOp := elapsed / time.Duration(i+1)
			remainingOps := totalOps - (i + 1)
			eta := avgTimePerOp * time.Duration(remainingOps)
			etaStr = formatDuration(eta)
		} else {
			etaStr = "--:--:--"
		}

		processedSizeStr := formatBytes(processedSize)
		totalSizeStr := formatBytes(totalSize)
		
		fmt.Printf("Processing '%s' partitions [%s] %3.0f%% | %s/%s | Elapsed: %s | ETA: %s\r", 
			partName, bar, progress, processedSizeStr, totalSizeStr, elapsedStr, etaStr)
	}

	totalTime := time.Since(startTime)
	totalTimeStr := formatDuration(totalTime)

	fmt.Printf("Processing '%s' partitions [%s] ✓ Done | %s | Time: %s          \n", 
		partName, strings.Repeat("=", 30), formatBytes(totalSize), totalTimeStr)

	return nil
}

func (d *Dumper) processOperation(op *pb.InstallOperation, outFile, oldFile *os.File, blockSize uint64) error {
	var data []byte
	if op.DataLength != nil && *op.DataLength > 0 {
		if _, err := d.payloadFile.Seek(d.dataOffset+int64(*op.DataOffset), io.SeekStart); err != nil {
			return err
		}
		data = make([]byte, *op.DataLength)
		if _, err := io.ReadFull(d.payloadFile, data); err != nil {
			return err
		}

		if op.DataSha256Hash != nil {
			hash := sha256.Sum256(data)
			if !bytes.Equal(hash[:], op.DataSha256Hash) {
				return fmt.Errorf("data hash mismatch")
			}
		}
	}

	return processOperationType(op, data, outFile, oldFile, blockSize)
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f%s", float64(bytes)/float64(div), units[exp])
}