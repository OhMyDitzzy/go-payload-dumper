package dumper

import (
    "bytes"
    "compress/bzip2"
    "encoding/binary"
    "fmt"
    "io"

    "github.com/klauspost/compress/zstd"
)

const (
    BSDIFF_MAGIC = "BSDIFF40"
    BSDF2_MAGIC  = "BSDF2"
)

func ApplyBSDIFF(oldData, patchData []byte) ([]byte, error) {
    reader := bytes.NewReader(patchData)

    magic := make([]byte, 8)
    if _, err := io.ReadFull(reader, magic); err != nil {
        return nil, err
    }

    var algControl, algDiff, algExtra int
    if string(magic[:8]) == BSDIFF_MAGIC {
        algControl, algDiff, algExtra = 1, 1, 1
    } else if string(magic[:5]) == BSDF2_MAGIC {
        algControl = int(magic[5])
        algDiff = int(magic[6])
        algExtra = int(magic[7])
    } else {
        return nil, fmt.Errorf("invalid bsdiff magic")
    }

    ctrlLen, err := readInt64(reader)
    if err != nil {
        return nil, err
    }
    diffLen, err := readInt64(reader)
    if err != nil {
        return nil, err
    }
    newSize, err := readInt64(reader)
    if err != nil {
        return nil, err
    }

    ctrlData := make([]byte, ctrlLen)
    if _, err := io.ReadFull(reader, ctrlData); err != nil {
        return nil, err
    }
    ctrlBlock, err := decompressBSDF2(algControl, ctrlData)
    if err != nil {
        return nil, err
    }

    diffData := make([]byte, diffLen)
    if _, err := io.ReadFull(reader, diffData); err != nil {
        return nil, err
    }
    diffBlock, err := decompressBSDF2(algDiff, diffData)
    if err != nil {
        return nil, err
    }

    extraData, err := io.ReadAll(reader)
    if err != nil {
        return nil, err
    }
    extraBlock, err := decompressBSDF2(algExtra, extraData)
    if err != nil {
        return nil, err
    }

    newData := make([]byte, newSize)
    oldPos, newPos := 0, 0
    diffPos, extraPos := 0, 0

    ctrlReader := bytes.NewReader(ctrlBlock)
    for newPos < int(newSize) {
        addSize, err := readInt64(ctrlReader)
        if err != nil {
            break
        }
        copySize, err := readInt64(ctrlReader)
        if err != nil {
            break
        }
        seekAmount, err := readInt64(ctrlReader)
        if err != nil {
            break
        }

        for i := 0; i < int(addSize); i++ {
            if oldPos+i < len(oldData) && diffPos+i < len(diffBlock) {
                newData[newPos+i] = oldData[oldPos+i] + diffBlock[diffPos+i]
            } else if diffPos+i < len(diffBlock) {
                newData[newPos+i] = diffBlock[diffPos+i]
            }
        }

        newPos += int(addSize)
        oldPos += int(addSize)
        diffPos += int(addSize)

        for i := 0; i < int(copySize); i++ {
            if extraPos+i < len(extraBlock) {
                newData[newPos+i] = extraBlock[extraPos+i]
            }
        }

        newPos += int(copySize)
        extraPos += int(copySize)
        oldPos += int(seekAmount)
    }

    return newData, nil
}

func decompressBSDF2(alg int, data []byte) ([]byte, error) {
    switch alg {
    case 0:
        return data, nil
    case 1:
        reader := bzip2.NewReader(bytes.NewReader(data))
        return io.ReadAll(reader)
    case 2:
        decoder, err := zstd.NewReader(bytes.NewReader(data))
        if err != nil {
            return nil, err
        }
        defer decoder.Close()
        return io.ReadAll(decoder)
    default:
        return nil, fmt.Errorf("unsupported compression algorithm: %d", alg)
    }
}

func readInt64(r io.Reader) (int64, error) {
    var val int64
    err := binary.Read(r, binary.LittleEndian, &val)
    return val, err
}