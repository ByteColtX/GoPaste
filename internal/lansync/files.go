package lansync

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopaste/internal/history"
)

// HTTP paths of the file transfer. One item takes two requests: the receiving
// device answers before any bytes are written, so a refusal reaches the sender as
// a reason instead of a broken connection (see SPEC.md, 文件项).
const (
	pathFilesOffer = "/v1/files/offer"
	// pathFilesUpload is followed by the token the offer reply handed out.
	pathFilesUpload = "/v1/files"
)

// The two shapes one file item travels in.
const (
	// fileFormRaw is a single regular file, sent as its own bytes. A LAN
	// transfer has bandwidth to spare, so compressing one file would only cost
	// time.
	fileFormRaw = "raw"
	// fileFormArchive is everything else — several paths, or a directory — as
	// one tar.gz stream. gzip earns its keep on the scattered small files a
	// folder copy carries.
	fileFormArchive = "archive"
)

// DefaultFileLimitBytes bounds one file item when no limit is configured: 2 GiB,
// which one LAN transfer finishes within minutes and one disk absorbs.
const DefaultFileLimitBytes int64 = 2 << 30

const (
	// fileStagingLifetime is how long an offered transfer may sit before the
	// staging sweep drops it. A sender that never uploads (crash, power loss)
	// must not leave a directory behind forever.
	fileStagingLifetime = 10 * time.Minute
	// fileUploadTimeout bounds one file upload on the sending side. It is
	// separate from requestTimeout because a large file legitimately takes
	// minutes, while a clipboard image does not.
	fileUploadTimeout = 30 * time.Minute
	// fileUploadStall is how long the receiving side waits for the next byte
	// before it gives up on an upload and cleans up.
	fileUploadStall = 60 * time.Second
	// maxOfferBytes bounds the offer request. An offer is a path list and a
	// size, so a megabyte is already absurdly generous.
	maxOfferBytes = 1 << 20
	// maxArchiveEntries bounds one archive, so a tar bomb cannot fill a disk with
	// empty files. It is far above what a copied folder carries in practice.
	maxArchiveEntries = 50000
	// archiveSlack is how many more bytes than the declared total the upload
	// stream may carry. gzip can grow incompressible input slightly; the
	// extracted size is checked against the declared total exactly.
	archiveSlack = 1 << 20
	// stagingDirName is the directory under FilesDir where an upload lands
	// before it is renamed into place.
	stagingDirName = ".staging"
)

// File transfer outcomes and reasons that are not shared with the clip endpoint.
const (
	// clipStatusGo means the receiving device agreed to take the upload.
	clipStatusGo = "go"

	// clipReasonUnsupported refuses a file item this device cannot store at all,
	// such as one received before a files directory was configured.
	clipReasonUnsupported = "unsupported"
	// clipReasonIncomplete reports an upload that did not arrive whole, or could
	// not be unpacked safely.
	clipReasonIncomplete = "incomplete"
)

var (
	// errFileTooLarge reports a payload bigger than the limit in force.
	errFileTooLarge = errors.New("lansync: the file payload is too large")
	// errArchiveUnsafePath reports an archive entry that would escape the
	// directory it is extracted into.
	errArchiveUnsafePath = errors.New("lansync: the archive entry points outside the item directory")
	// errArchiveTooManyEntries reports an archive with more entries than one
	// item may carry.
	errArchiveTooManyEntries = errors.New("lansync: the archive carries too many entries")
	// errArchiveUnsupported reports an archive entry of a kind this transfer
	// cannot represent, such as a device node.
	errArchiveUnsupported = errors.New("lansync: the archive carries an unsupported entry")
)

// fileOffer is the sending device's declaration of one file item: what it is
// about to send, before any of it travels.
type fileOffer struct {
	Version string `json:"version"`
	Kind    int    `json:"kind"`
	// Paths are the paths the sending device copied, in clipboard order. They
	// are the item's identity: the receiving device derives the content ID from
	// them.
	Paths []string `json:"paths"`
	// Form is fileFormRaw or fileFormArchive.
	Form string `json:"form"`
	// Name is the file name a raw form carries.
	Name string `json:"name,omitempty"`
	// Total is the byte count of every regular file the item carries, before
	// compression. Both sides check the size limit against it.
	Total int64 `json:"total"`
}

// fileOfferReply answers an offer: go with the token to upload to, or the reason
// it was refused.
type fileOfferReply struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Token  string `json:"token,omitempty"`
}

// fileTransfer is one upload the receiving device has agreed to take.
type fileTransfer struct {
	deviceID string
	offer    fileOffer
	// dir is the staging directory the upload lands in. It becomes the item's
	// directory once every byte is there and has been unpacked.
	dir string
	// root is the files directory this transfer started in. An upload keeps
	// landing where it began even if the user picks another directory while it
	// runs: the staging directory cannot be renamed across filesystems.
	root      string
	createdAt time.Time
}

// filePayload is what one file item is about to send.
type filePayload struct {
	form string
	// paths are the paths the sending device copied, in clipboard order.
	paths []string
	// name is the file name a raw payload carries.
	name string
	// total is the uncompressed byte count of every regular file carried.
	total int64
}

// filePayloadOf works out how a file clip travels: a single regular file as
// itself, anything else as one archive. It fails when a source path cannot be
// read, because there is nothing to send then.
func filePayloadOf(clip Clip) (filePayload, error) {
	if len(clip.Files) == 0 {
		return filePayload{}, errors.New("lansync: the file item has no paths")
	}
	payload := filePayload{paths: clip.Files}
	if len(clip.Files) == 1 {
		info, err := os.Lstat(clip.Files[0])
		if err != nil {
			return filePayload{}, err
		}
		if info.Mode().IsRegular() {
			payload.form = fileFormRaw
			payload.name = filepath.Base(clip.Files[0])
			payload.total = info.Size()
			return payload, nil
		}
	}
	total, err := filePayloadSize(clip.Files)
	if err != nil {
		return filePayload{}, err
	}
	payload.form = fileFormArchive
	payload.total = total
	return payload, nil
}

// filePayloadSize sums the regular file bytes under every path, which is the
// size the limit is checked against on both sides.
func filePayloadSize(paths []string) (int64, error) {
	var total int64
	for _, source := range paths {
		err := filepath.WalkDir(source, func(current string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				total += info.Size()
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

// deliverFiles delivers one file item to every target and reports one outcome
// each. A payload that cannot be read, or that is bigger than the limit, is
// settled here without touching the network.
func (s *Service) deliverFiles(clip Clip, targets []Device) []history.DeviceDelivery {
	settle := func(state history.DeliveryState) []history.DeviceDelivery {
		deliveries := make([]history.DeviceDelivery, len(targets))
		for index, device := range targets {
			deliveries[index] = history.DeviceDelivery{DeviceID: device.ID, State: state}
		}
		return deliveries
	}
	payload, err := filePayloadOf(clip)
	if err != nil {
		// 源文件读不出来（复制后被删了、权限变了）：不是网络问题，也不能靠
		// 重试变好，所以记「未送达」，等用户自己重发。
		return settle(history.DeliveryPending)
	}
	if payload.total > s.fileLimitBytes() {
		return settle(history.DeliveryTooLarge)
	}

	deliveries := make([]history.DeviceDelivery, len(targets))
	var wg sync.WaitGroup
	for index, device := range targets {
		index, device := index, device
		wg.Add(1)
		go func() {
			defer wg.Done()
			deliveries[index] = history.DeviceDelivery{
				DeviceID: device.ID,
				State:    s.deliverFilesTo(payload, device),
			}
		}()
	}
	wg.Wait()
	return deliveries
}

// deliverFilesTo offers one file payload to one device and, when it is taken,
// uploads it.
func (s *Service) deliverFilesTo(payload filePayload, device Device) history.DeliveryState {
	if !device.Online || device.Addr == nil {
		return history.DeliveryOffline
	}
	baseURL := "https://" + net.JoinHostPort(device.Addr.String(), fmt.Sprint(device.Port))
	offer := fileOffer{
		Version: clipProtocolVersion,
		Kind:    int(history.KindFile),
		Paths:   payload.paths,
		Form:    payload.form,
		Name:    payload.name,
		Total:   payload.total,
	}
	var reply fileOfferReply
	status, err := s.postOffer(device, baseURL+pathFilesOffer, offer, &reply)
	if err != nil {
		// The device is not answering where discovery saw it: treat it as gone
		// until it is seen again, exactly like a failed clip delivery.
		s.markUnreachable(device.ID)
		return history.DeliveryOffline
	}
	if status != http.StatusOK {
		// A peer that does not know this endpoint answers 404: nothing was
		// delivered, and nothing will be until it runs this build.
		return history.DeliveryPending
	}
	if reply.Status != clipStatusGo || reply.Token == "" {
		if reply.Reason == clipReasonSize {
			return history.DeliveryTooLarge
		}
		return history.DeliveryPending
	}
	return s.uploadFiles(payload, device, baseURL+pathFilesUpload+"/"+reply.Token)
}

// postOffer sends one offer and decodes the reply.
func (s *Service) postOffer(device Device, url string, offer fileOffer, out *fileOfferReply) (int, error) {
	payload, err := json.Marshal(offer)
	if err != nil {
		return 0, err
	}
	s.mu.RLock()
	serviceCtx := s.ctx
	s.mu.RUnlock()
	if serviceCtx == nil {
		return 0, ErrNoService
	}
	ctx, cancel := context.WithTimeout(serviceCtx, requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return response.StatusCode, nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<16)).Decode(out); err != nil {
		return response.StatusCode, err
	}
	return response.StatusCode, nil
}

// uploadFiles streams one file payload to a device that agreed to take it.
func (s *Service) uploadFiles(payload filePayload, device Device, url string) history.DeliveryState {
	body, length, err := payload.body()
	if err != nil {
		return history.DeliveryPending
	}
	defer body.Close()

	s.mu.RLock()
	serviceCtx := s.ctx
	s.mu.RUnlock()
	if serviceCtx == nil {
		return history.DeliveryOffline
	}
	ctx, cancel := context.WithTimeout(serviceCtx, fileUploadTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return history.DeliveryPending
	}
	request.ContentLength = length
	request.Header.Set("Content-Type", payload.contentType())
	response, err := s.fileClient.Do(request)
	if err != nil {
		s.markUnreachable(device.ID)
		return history.DeliveryOffline
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return history.DeliveryPending
	}
	var reply clipReply
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<16)).Decode(&reply); err != nil {
		return history.DeliveryOffline
	}
	switch {
	case reply.Status == clipStatusReceived:
		s.markReachable(device.ID)
		return history.DeliveryReceived
	case reply.Reason == clipReasonSize:
		return history.DeliveryTooLarge
	default:
		return history.DeliveryPending
	}
}

// body returns the bytes to upload and their length, or -1 when the length is
// not known up front, as an archive's compressed size is not.
func (p filePayload) body() (io.ReadCloser, int64, error) {
	if p.form == fileFormRaw {
		file, err := os.Open(p.paths[0])
		if err != nil {
			return nil, 0, err
		}
		return file, p.total, nil
	}
	reader, writer := io.Pipe()
	go func() {
		err := writeArchive(writer, p.paths)
		// CloseWithError hands a read or archive error to the HTTP client, which
		// then aborts the request instead of sending a truncated archive that
		// looks complete.
		writer.CloseWithError(err)
	}()
	return reader, -1, nil
}

// contentType is the media type of the upload body.
func (p filePayload) contentType() string {
	if p.form == fileFormRaw {
		return "application/octet-stream"
	}
	return "application/gzip"
}

// writeArchive writes every source path as one tar.gz stream, in clipboard
// order. Entries are named after the copied path's own name, so the receiving
// device gets back the directory structure the user copied.
func writeArchive(w io.Writer, sources []string) error {
	compressed := gzip.NewWriter(w)
	archive := tar.NewWriter(compressed)
	names := newArchiveNames()
	for _, source := range sources {
		if err := writeArchiveSource(archive, names, source); err != nil {
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	return compressed.Close()
}

// writeArchiveSource adds one copied path, and everything under it, to the
// archive.
func writeArchiveSource(archive *tar.Writer, names *archiveNames, source string) error {
	parent := filepath.Dir(source)
	top := names.unique(filepath.Base(source))
	return filepath.WalkDir(source, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(parent, current)
		if err != nil {
			return err
		}
		parts := strings.Split(relative, string(filepath.Separator))
		parts[0] = top
		name := path.Join(parts...)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()
		switch {
		case mode.IsDir():
			return archive.WriteHeader(&tar.Header{
				Typeflag: tar.TypeDir,
				Name:     name + "/",
				Mode:     0o700,
				ModTime:  info.ModTime(),
			})
		case mode.IsRegular():
			if err := archive.WriteHeader(&tar.Header{
				Typeflag: tar.TypeReg,
				Name:     name,
				Mode:     archiveFileMode(mode),
				Size:     info.Size(),
				ModTime:  info.ModTime(),
			}); err != nil {
				return err
			}
			file, err := os.Open(current)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.CopyN(archive, file, info.Size())
			return err
		case mode&os.ModeSymlink != 0:
			target, err := os.Readlink(current)
			if err != nil {
				return err
			}
			return archive.WriteHeader(&tar.Header{
				Typeflag: tar.TypeSymlink,
				Name:     name,
				Linkname: target,
				Mode:     0o777,
			})
		default:
			// 设备文件、管道、socket 没有可传的内容，跳过而不是让整条失败。
			return nil
		}
	})
}

// archiveFileMode keeps only the executable bit: the payload is clipboard
// content, not a backup, so the owner's permissions are all it needs.
func archiveFileMode(mode fs.FileMode) int64 {
	if mode&0o111 != 0 {
		return 0o700
	}
	return 0o600
}

// archiveNames hands out unique top-level names, so copying two files with the
// same name from different directories does not collapse them into one.
type archiveNames struct {
	used map[string]bool
}

func newArchiveNames() *archiveNames {
	return &archiveNames{used: map[string]bool{}}
}

func (n *archiveNames) unique(base string) string {
	if base == "" {
		base = "file"
	}
	if !n.used[base] {
		n.used[base] = true
		return base
	}
	extension := filepath.Ext(base)
	stem := strings.TrimSuffix(base, extension)
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s (%d)%s", stem, index, extension)
		if !n.used[candidate] {
			n.used[candidate] = true
			return candidate
		}
	}
}

// handleFileOffer accepts or refuses one file item before its bytes travel.
func (s *Service) handleFileOffer(w http.ResponseWriter, r *http.Request) {
	device, ok := pairedPeer(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	var offer fileOffer
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxOfferBytes)).Decode(&offer); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	root := s.filesDirPath()
	refuse := func(reason string) {
		writeJSON(w, http.StatusOK, fileOfferReply{Status: clipStatusIgnored, Reason: reason})
	}
	switch {
	case offer.Version != clipProtocolVersion:
		refuse(clipReasonVersion)
		return
	case history.Kind(offer.Kind) != history.KindFile:
		refuse(clipReasonKind)
		return
	case len(offer.Paths) == 0:
		refuse(clipReasonKind)
		return
	case offer.Form != fileFormRaw && offer.Form != fileFormArchive:
		refuse(clipReasonKind)
		return
	case offer.Form == fileFormRaw && !safeFileName(offer.Name):
		refuse(clipReasonKind)
		return
	case !s.syncOn():
		// The master switch is off: refuse like any other setting that does not
		// allow this content, so the sending device records a refusal instead
		// of a delivery.
		refuse(clipReasonSync)
		return
	case !s.fileSyncOn():
		refuse(clipReasonSync)
		return
	case root == "":
		// 没有落盘目录就收不了文件：如实拒收，而不是收下一半。
		refuse(clipReasonUnsupported)
		return
	case !device.Receives(ContentFile):
		refuse(clipReasonDirection)
		return
	case offer.Total < 0 || offer.Total > s.fileLimitBytes():
		refuse(clipReasonSize)
		return
	}

	s.sweepFileStaging()
	token, err := newFileToken()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	dir := filepath.Join(root, stagingDirName, token)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	s.filesMu.Lock()
	s.files[token] = &fileTransfer{
		deviceID:  device.ID,
		offer:     offer,
		dir:       dir,
		root:      root,
		createdAt: s.now(),
	}
	s.filesMu.Unlock()
	writeJSON(w, http.StatusOK, fileOfferReply{Status: clipStatusGo, Token: token})
}

// handleFileUpload takes the bytes of one offered file item, stores them, and
// reports the payload to the application.
func (s *Service) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	device, ok := pairedPeer(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	transfer, ok := s.takeFileTransfer(r.PathValue("token"), device.ID)
	if !ok {
		// An unknown, expired, or foreign token: nothing to write to.
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	top, err := s.receiveFileBody(w, transfer, r)
	if err == nil {
		var clip Clip
		clip, err = s.installFileTransfer(transfer, top)
		if err == nil {
			if s.options.OnClip != nil {
				s.options.OnClip(clip, device)
			}
			s.markReachable(device.ID)
			writeJSON(w, http.StatusOK, clipReply{Status: clipStatusReceived})
			return
		}
	}
	os.RemoveAll(transfer.dir)
	if errors.Is(err, errFileTooLarge) {
		writeJSON(w, http.StatusOK, clipReply{Status: clipStatusIgnored, Reason: clipReasonSize})
		return
	}
	writeJSON(w, http.StatusOK, clipReply{Status: clipStatusIgnored, Reason: clipReasonIncomplete})
}

// receiveFileBody streams an upload into the transfer's staging directory and
// returns the names of the copied paths, relative to it.
func (s *Service) receiveFileBody(w http.ResponseWriter, transfer *fileTransfer, r *http.Request) ([]string, error) {
	// A raw file carries exactly its own bytes; an archive is compressed, so it
	// may be a little bigger than the declared total but never much.
	limit := transfer.offer.Total
	if transfer.offer.Form == fileFormArchive {
		limit += archiveSlack
	}
	guard := newStallGuard(http.MaxBytesReader(w, r.Body, limit), fileUploadStall)
	defer guard.Close()

	if transfer.offer.Form == fileFormRaw {
		target := filepath.Join(transfer.dir, transfer.offer.Name)
		file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, err
		}
		written, copyErr := io.Copy(file, io.LimitReader(guard, transfer.offer.Total+1))
		closeErr := file.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if written != transfer.offer.Total {
			return nil, fmt.Errorf("lansync: the upload carried %d of %d bytes", written, transfer.offer.Total)
		}
		return []string{transfer.offer.Name}, nil
	}
	return extractArchive(transfer.dir, guard, transfer.offer.Total)
}

// installFileTransfer moves a finished staging directory into the item's own
// directory and returns the payload the application stores.
func (s *Service) installFileTransfer(transfer *fileTransfer, top []string) (Clip, error) {
	id := history.ContentID(history.Content{Kind: history.KindFile, Files: transfer.offer.Paths})
	final := filepath.Join(transfer.root, id)
	var local []string
	if existing, ok := topLevelNames(final); ok {
		// The same content is already here: drop the copy just uploaded and use
		// the files that are on disk, so one item never owns two directories.
		os.RemoveAll(transfer.dir)
		local = topLevelPaths(final, existing)
	} else {
		if err := os.MkdirAll(transfer.root, 0o700); err != nil {
			return Clip{}, err
		}
		if err := os.Rename(transfer.dir, final); err != nil {
			// Two devices sending the same content at once: the other one got
			// there first, so its files are the ones to use.
			existing, ok := topLevelNames(final)
			if !ok {
				return Clip{}, err
			}
			os.RemoveAll(transfer.dir)
			local = topLevelPaths(final, existing)
		} else {
			local = topLevelPaths(final, top)
		}
	}
	return Clip{
		Kind:       history.KindFile,
		Files:      transfer.offer.Paths,
		LocalFiles: local,
		Preview:    s.previewOf(local),
		FileSize:   directorySize(final),
		CreatedAt:  transfer.createdAt,
	}, nil
}

// previewOf builds the preview for a payload that just landed, from the first
// file the item holds. It runs on the transfer goroutine: the interface must
// never wait for a decode, and an item that arrived with its preview needs no
// second pass over the history.
func (s *Service) previewOf(local []string) *history.FilePreview {
	if s.options.PreviewFile == nil || len(local) == 0 {
		return nil
	}
	return s.options.PreviewFile(local[0])
}

// topLevelPaths joins the names of an item directory's top-level entries onto it.
func topLevelPaths(dir string, names []string) []string {
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(dir, name))
	}
	return paths
}

// takeFileTransfer removes one offered transfer from the map, so a token is good
// for exactly one upload and only from the device it was offered to.
func (s *Service) takeFileTransfer(token, deviceID string) (*fileTransfer, bool) {
	if token == "" {
		return nil, false
	}
	s.filesMu.Lock()
	defer s.filesMu.Unlock()
	transfer, ok := s.files[token]
	if !ok || transfer.deviceID != deviceID {
		return nil, false
	}
	delete(s.files, token)
	return transfer, true
}

// sweepFileStaging drops offers nobody uploaded and directories left behind by a
// crashed run, so the staging area cannot grow without bound.
func (s *Service) sweepFileStaging() {
	root := filepath.Join(s.filesDirPath(), stagingDirName)
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := s.now().Add(-fileStagingLifetime)
	s.filesMu.Lock()
	defer s.filesMu.Unlock()
	for token, transfer := range s.files {
		if transfer.createdAt.Before(cutoff) {
			delete(s.files, token)
			os.RemoveAll(transfer.dir)
		}
	}
	for _, entry := range entries {
		if _, pending := s.files[entry.Name()]; pending {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		os.RemoveAll(filepath.Join(root, entry.Name()))
	}
}

// extractArchive unpacks one tar.gz stream into dir and returns the names of the
// copied paths, relative to it, in the order the archive carries them.
//
// Nothing an archive names may land outside dir, and a symlink is created only
// after every regular entry, so no entry is ever written through a link that
// points somewhere else.
func extractArchive(dir string, stream io.Reader, limit int64) ([]string, error) {
	compressed, err := gzip.NewReader(stream)
	if err != nil {
		return nil, err
	}
	defer compressed.Close()

	archive := tar.NewReader(compressed)
	seen := map[string]bool{}
	top := make([]string, 0, 4)
	type link struct{ name, target string }
	links := make([]link, 0, 4)
	var total int64
	entries := 0

	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		entries++
		if entries > maxArchiveEntries {
			return nil, errArchiveTooManyEntries
		}
		name, err := archiveEntryPath(header.Name)
		if err != nil {
			return nil, err
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		if !insideDirectory(dir, target) {
			return nil, errArchiveUnsafePath
		}
		record := func() {
			root := strings.SplitN(name, "/", 2)[0]
			if !seen[root] {
				seen[root] = true
				top = append(top, root)
			}
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return nil, err
			}
			record()
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return nil, err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				return nil, err
			}
			written, copyErr := io.CopyN(file, archive, header.Size)
			closeErr := file.Close()
			total += written
			if copyErr != nil {
				return nil, copyErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
			if total > limit {
				return nil, errFileTooLarge
			}
			if header.Mode&0o111 != 0 {
				os.Chmod(target, 0o700)
			}
			record()
		case tar.TypeSymlink:
			links = append(links, link{name: target, target: header.Linkname})
			record()
		default:
			return nil, errArchiveUnsupported
		}
	}
	for _, pending := range links {
		if err := os.MkdirAll(filepath.Dir(pending.name), 0o700); err != nil {
			return nil, err
		}
		if err := os.Symlink(pending.target, pending.name); err != nil {
			return nil, err
		}
	}
	return top, nil
}

// archiveEntryPath cleans one archive entry name and rejects anything that could
// escape the directory it is extracted into.
func archiveEntryPath(name string) (string, error) {
	if name == "" || strings.ContainsRune(name, '\\') {
		return "", errArchiveUnsafePath
	}
	cleaned := path.Clean(strings.TrimSuffix(name, "/"))
	switch {
	case cleaned == "" || cleaned == "." || cleaned == "..":
		return "", errArchiveUnsafePath
	case path.IsAbs(cleaned), strings.HasPrefix(cleaned, "../"):
		return "", errArchiveUnsafePath
	}
	return cleaned, nil
}

// insideDirectory reports whether target is dir itself or something below it.
func insideDirectory(dir, target string) bool {
	relative, err := filepath.Rel(dir, target)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// safeFileName reports whether a raw file name can be created inside an item
// directory as-is.
func safeFileName(name string) bool {
	switch name {
	case "", ".", "..":
		return false
	}
	return !strings.ContainsAny(name, `/\`) && !strings.ContainsRune(name, 0)
}

// topLevelNames lists what an item directory holds, or reports false when the
// directory is missing or empty.
func topLevelNames(dir string) ([]string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return nil, false
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, true
}

// directorySize is the byte size of the regular files under dir, which is what a
// received file item reports.
func directorySize(dir string) int64 {
	var total int64
	filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// newFileToken returns the unguessable name one offered transfer is uploaded to.
func newFileToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

// stallGuard closes an upload body that stops making progress, so a peer that
// goes quiet mid-transfer cannot hold a staging directory and a goroutine
// forever. Closing the body is what makes the blocked read return.
type stallGuard struct {
	body  io.ReadCloser
	timer *time.Timer
	limit time.Duration
}

func newStallGuard(body io.ReadCloser, limit time.Duration) *stallGuard {
	guard := &stallGuard{body: body, limit: limit}
	guard.timer = time.AfterFunc(limit, func() { body.Close() })
	return guard
}

func (g *stallGuard) Read(p []byte) (int, error) {
	read, err := g.body.Read(p)
	if read > 0 {
		g.timer.Reset(g.limit)
	}
	return read, err
}

func (g *stallGuard) Close() error {
	g.timer.Stop()
	return g.body.Close()
}
