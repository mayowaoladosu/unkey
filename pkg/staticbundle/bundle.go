// Package staticbundle packages framework build output into deterministic,
// bounded archives and resolves safe HTTP paths from them.
package staticbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

type Limits struct {
	MaxArchiveBytes      int64
	MaxFiles             int
	MaxFileBytes         int64
	MaxUncompressedBytes int64
}

func DefaultLimits() Limits {
	return Limits{
		MaxArchiveBytes:      256 << 20,
		MaxFiles:             50_000,
		MaxFileBytes:         64 << 20,
		MaxUncompressedBytes: 1 << 30,
	}
}

type Bundle struct {
	Bytes             []byte
	Digest            string
	FileCount         int
	UncompressedBytes int64
}

type File struct {
	Body         []byte
	ContentType  string
	CacheControl string
	ETag         string
}

type Archive struct {
	files map[string]File
}

func PackDirectory(root string, limits Limits) (Bundle, error) {
	paths := make([]string, 0)
	var uncompressedBytes int64

	err := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filePath == root || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("static bundle does not allow symlink %q", filePath)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("static bundle only supports regular files: %q", filePath)
		}
		if info.Size() > limits.MaxFileBytes {
			return fmt.Errorf("static file %q exceeds %d bytes", filePath, limits.MaxFileBytes)
		}
		uncompressedBytes += info.Size()
		if uncompressedBytes > limits.MaxUncompressedBytes {
			return fmt.Errorf("static bundle exceeds %d uncompressed bytes", limits.MaxUncompressedBytes)
		}
		paths = append(paths, filePath)
		if len(paths) > limits.MaxFiles {
			return fmt.Errorf("static bundle exceeds %d files", limits.MaxFiles)
		}
		return nil
	})
	if err != nil {
		return Bundle{}, fmt.Errorf("scan static output: %w", err)
	}
	slices.Sort(paths)

	var encoded bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&encoded, gzip.BestCompression)
	if err != nil {
		return Bundle{}, fmt.Errorf("create gzip writer: %w", err)
	}
	gzipWriter.Header.ModTime = time.Unix(0, 0)
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)

	for _, filePath := range paths {
		relativePath, err := filepath.Rel(root, filePath)
		if err != nil {
			return Bundle{}, fmt.Errorf("resolve static path: %w", err)
		}
		relativePath = filepath.ToSlash(relativePath)
		body, err := os.ReadFile(filePath)
		if err != nil {
			return Bundle{}, fmt.Errorf("read static file %q: %w", relativePath, err)
		}
		header := &tar.Header{
			Name:       relativePath,
			Mode:       0o644,
			Size:       int64(len(body)),
			Typeflag:   tar.TypeReg,
			ModTime:    time.Unix(0, 0),
			AccessTime: time.Unix(0, 0),
			ChangeTime: time.Unix(0, 0),
			Format:     tar.FormatPAX,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return Bundle{}, fmt.Errorf("write static header %q: %w", relativePath, err)
		}
		if _, err := tarWriter.Write(body); err != nil {
			return Bundle{}, fmt.Errorf("write static file %q: %w", relativePath, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		return Bundle{}, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return Bundle{}, fmt.Errorf("close gzip writer: %w", err)
	}
	if int64(encoded.Len()) > limits.MaxArchiveBytes {
		return Bundle{}, fmt.Errorf("static archive exceeds %d bytes", limits.MaxArchiveBytes)
	}

	body := encoded.Bytes()
	digest := sha256.Sum256(body)
	return Bundle{
		Bytes:             append([]byte(nil), body...),
		Digest:            hex.EncodeToString(digest[:]),
		FileCount:         len(paths),
		UncompressedBytes: uncompressedBytes,
	}, nil
}

func Open(encoded []byte, limits Limits) (*Archive, error) {
	if int64(len(encoded)) > limits.MaxArchiveBytes {
		return nil, fmt.Errorf("static archive exceeds %d bytes", limits.MaxArchiveBytes)
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("open static gzip: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	archive := &Archive{files: make(map[string]File)}
	tarReader := tar.NewReader(gzipReader)
	var totalBytes int64
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read static archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf("static archive contains unsupported entry %q", header.Name)
		}
		name, ok := safeArchivePath(header.Name)
		if !ok {
			return nil, fmt.Errorf("static archive contains unsafe path %q", header.Name)
		}
		if _, exists := archive.files[name]; exists {
			return nil, fmt.Errorf("static archive contains duplicate path %q", name)
		}
		if header.Size < 0 || header.Size > limits.MaxFileBytes {
			return nil, fmt.Errorf("static file %q exceeds %d bytes", name, limits.MaxFileBytes)
		}
		totalBytes += header.Size
		if totalBytes > limits.MaxUncompressedBytes {
			return nil, fmt.Errorf("static archive exceeds %d uncompressed bytes", limits.MaxUncompressedBytes)
		}
		if len(archive.files) >= limits.MaxFiles {
			return nil, fmt.Errorf("static archive exceeds %d files", limits.MaxFiles)
		}

		body, err := io.ReadAll(io.LimitReader(tarReader, limits.MaxFileBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read static file %q: %w", name, err)
		}
		if int64(len(body)) != header.Size {
			return nil, fmt.Errorf("static file %q has invalid size", name)
		}
		archive.files[name] = newFile(name, body)
	}
	return archive, nil
}

func (a *Archive) Resolve(requestPath string, spaFallback bool) (File, bool) {
	name, ok := safeRequestPath(requestPath)
	if !ok {
		return File{}, false
	}
	candidates := []string{name}
	if name == "" {
		candidates = []string{"index.html"}
	} else if path.Ext(name) == "" {
		candidates = append(candidates, path.Join(name, "index.html"))
		if spaFallback {
			candidates = append(candidates, "index.html")
		}
	}
	for _, candidate := range candidates {
		file, found := a.files[candidate]
		if found {
			file.Body = append([]byte(nil), file.Body...)
			return file, true
		}
	}
	return File{}, false
}

var immutableHash = regexp.MustCompile(`(?i)(?:^|[._-])[a-f0-9]{8,}(?:[._-]|$)`)

func newFile(name string, body []byte) File {
	contentType := mime.TypeByExtension(path.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	cacheControl := "public, max-age=3600"
	if strings.HasSuffix(strings.ToLower(name), ".html") {
		contentType = "text/html; charset=utf-8"
		cacheControl = "no-cache"
	} else if immutableHash.MatchString(path.Base(name)) {
		cacheControl = "public, max-age=31536000, immutable"
	}
	digest := sha256.Sum256(body)
	return File{
		Body:         append([]byte(nil), body...),
		ContentType:  contentType,
		CacheControl: cacheControl,
		ETag:         hex.EncodeToString(digest[:]),
	}
}

func safeArchivePath(value string) (string, bool) {
	if strings.Contains(value, `\`) || strings.HasPrefix(value, "/") {
		return "", false
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return cleaned, true
}

func safeRequestPath(value string) (string, bool) {
	decoded, err := url.PathUnescape(value)
	if err != nil || strings.Contains(decoded, `\`) {
		return "", false
	}
	for _, segment := range strings.Split(decoded, "/") {
		if segment == ".." {
			return "", false
		}
	}
	cleaned := strings.TrimPrefix(path.Clean("/"+decoded), "/")
	if cleaned == "." {
		return "", true
	}
	return cleaned, true
}
