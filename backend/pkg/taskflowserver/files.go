package taskflowserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

func (s *Server) workspacePath(vmID, requested string) (string, error) {
	if !safeIDPattern.MatchString(vmID) {
		return "", fmt.Errorf("invalid VM ID")
	}
	root := filepath.Join(s.cfg.WorkspaceRoot, vmID)
	cleaned := strings.TrimSpace(requested)
	cleaned = strings.TrimPrefix(cleaned, "file://")
	cleaned = strings.TrimPrefix(cleaned, "/workspace")
	cleaned = strings.TrimPrefix(cleaned, "~/workspace")
	cleaned = strings.TrimLeft(cleaned, "/\\")
	target := filepath.Clean(filepath.Join(root, filepath.FromSlash(cleaned)))
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return target, nil
}

func fileKind(info os.FileInfo) taskflow.FileKind {
	if info.Mode()&os.ModeSymlink != 0 {
		return taskflow.FileKindSymlink
	}
	if info.IsDir() {
		return taskflow.FileKindDir
	}
	return taskflow.FileKindFile
}

func describeFile(path string, info os.FileInfo) *taskflow.File {
	return &taskflow.File{
		Name: info.Name(), User: "devloom", Size: uint64(info.Size()), Kind: fileKind(info),
		UnixMode: uint32(info.Mode().Perm()), CreatedAt: info.ModTime().Unix(), AccessedAt: info.ModTime().Unix(), UpdatedAt: info.ModTime().Unix(),
	}
}

func (s *Server) files(w http.ResponseWriter, r *http.Request) {
	var req taskflow.FileReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if registration, remote := s.remoteVM(req.ID); remote {
		var result []*taskflow.File
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/files", &req, &result); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, result)
		return
	}
	target, err := s.workspacePath(req.ID, req.Path)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	switch req.Operate {
	case taskflow.FileOpList:
		entries, readErr := os.ReadDir(target)
		if readErr != nil {
			fail(w, statusFor(readErr), readErr)
			return
		}
		files := make([]*taskflow.File, 0, len(entries))
		for _, entry := range entries {
			info, statErr := entry.Info()
			if statErr == nil {
				files = append(files, describeFile(filepath.Join(target, entry.Name()), info))
			}
		}
		sort.Slice(files, func(i, j int) bool {
			if files[i].Kind != files[j].Kind {
				return files[i].Kind == taskflow.FileKindDir
			}
			return files[i].Name < files[j].Name
		})
		respond(w, http.StatusOK, files)
	case taskflow.FileOpSave:
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		if err := os.WriteFile(target, []byte(req.Content), 0o640); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		respond(w, http.StatusOK, []*taskflow.File{})
	case taskflow.FileOpMkdir:
		if err := os.MkdirAll(target, 0o750); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		respond(w, http.StatusOK, []*taskflow.File{})
	case taskflow.FileOpDelete:
		if target == filepath.Join(s.cfg.WorkspaceRoot, req.ID) {
			fail(w, http.StatusBadRequest, fmt.Errorf("cannot delete workspace root"))
			return
		}
		if err := os.RemoveAll(target); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		respond(w, http.StatusOK, []*taskflow.File{})
	case taskflow.FileOpCopy, taskflow.FileOpMove:
		source, sourceErr := s.workspacePath(req.ID, req.Source)
		if sourceErr != nil {
			fail(w, http.StatusBadRequest, sourceErr)
			return
		}
		destination, destinationErr := s.workspacePath(req.ID, req.Target)
		if destinationErr != nil {
			fail(w, http.StatusBadRequest, destinationErr)
			return
		}
		if req.Operate == taskflow.FileOpMove {
			err = os.Rename(source, destination)
		} else {
			err = copyTree(source, destination)
		}
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		respond(w, http.StatusOK, []*taskflow.File{})
	default:
		fail(w, http.StatusBadRequest, fmt.Errorf("unsupported file operation %q", req.Operate))
	}
}

func copyTree(source, destination string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request) {
	if registration, remote := s.remoteVM(r.URL.Query().Get("id")); remote {
		s.proxyRunner(w, r, registration)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")
	path, err := s.workspacePath(r.URL.Query().Get("id"), r.URL.Query().Get("path"))
	if err != nil {
		_ = conn.Write(r.Context(), websocket.MessageText, []byte("ERR:"+err.Error()))
		return
	}
	file, err := os.Open(path)
	if err != nil {
		_ = conn.Write(r.Context(), websocket.MessageText, []byte("ERR:"+err.Error()))
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		_ = conn.Write(r.Context(), websocket.MessageText, []byte("ERR:"+err.Error()))
		return
	}
	_ = conn.Write(r.Context(), websocket.MessageText, []byte(fmt.Sprintf("SIZE:%d", info.Size())))
	buffer := make([]byte, 64<<10)
	for {
		count, readErr := file.Read(buffer)
		if count > 0 {
			if err := conn.Write(r.Context(), websocket.MessageBinary, buffer[:count]); err != nil {
				return
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = conn.Write(r.Context(), websocket.MessageText, []byte("ERR:"+readErr.Error()))
			return
		}
	}
	_ = conn.Write(r.Context(), websocket.MessageText, []byte("DONE"))
}

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	if registration, remote := s.remoteVM(r.URL.Query().Get("id")); remote {
		s.proxyRunner(w, r, registration)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")
	path, err := s.workspacePath(r.URL.Query().Get("id"), r.URL.Query().Get("path"))
	if err != nil {
		_ = conn.Write(r.Context(), websocket.MessageText, []byte("ERR:"+err.Error()))
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		_ = conn.Write(r.Context(), websocket.MessageText, []byte("ERR:"+err.Error()))
		return
	}
	temporary := path + ".upload"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		_ = conn.Write(r.Context(), websocket.MessageText, []byte("ERR:"+err.Error()))
		return
	}
	defer os.Remove(temporary)
	for {
		kind, data, readErr := conn.Read(r.Context())
		if readErr != nil {
			file.Close()
			return
		}
		if kind == websocket.MessageText && string(data) == "EOF" {
			break
		}
		if kind == websocket.MessageBinary {
			if _, err := file.Write(data); err != nil {
				file.Close()
				_ = conn.Write(r.Context(), websocket.MessageText, []byte("ERR:"+err.Error()))
				return
			}
		}
	}
	if err := file.Close(); err != nil {
		_ = conn.Write(r.Context(), websocket.MessageText, []byte("ERR:"+err.Error()))
		return
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = conn.Write(r.Context(), websocket.MessageText, []byte("ERR:"+err.Error()))
		return
	}
	_ = conn.Write(r.Context(), websocket.MessageText, []byte("DONE"))
}

func (s *Server) vmForTask(taskID string) (string, error) {
	vmID := s.taskVMID(taskID)
	if vmID == "" {
		return "", fmt.Errorf("task %s has no VM", taskID)
	}
	return vmID, nil
}

func (s *Server) repoListFiles(w http.ResponseWriter, r *http.Request) {
	var req taskflow.RepoListFilesReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	vmID, err := s.vmForTask(req.TaskId)
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	if registration, remote := s.remoteVM(vmID); remote {
		var result taskflow.RepoListFiles
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/task/repo-list-files", &req, &result); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, &result)
		return
	}
	path, err := s.workspacePath(vmID, req.Path)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		fail(w, statusFor(err), err)
		return
	}
	result := &taskflow.RepoListFiles{TaskId: req.TaskId, RequestId: req.RequestId, Path: req.Path, Success: true}
	for _, entry := range entries {
		if !req.IncludeHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		mode := taskflow.RepoEntryModeFile
		if info.IsDir() {
			mode = taskflow.RepoEntryModeTree
		} else if info.Mode()&0o111 != 0 {
			mode = taskflow.RepoEntryModeExecutable
		}
		itemPath := filepath.ToSlash(filepath.Join(req.Path, entry.Name()))
		result.Files = append(result.Files, &taskflow.RepoFileInfo{Name: entry.Name(), Path: itemPath, EntryMode: mode, Size: info.Size(), ModifiedAt: info.ModTime().UnixMilli()})
	}
	respond(w, http.StatusOK, result)
}

func (s *Server) repoReadFile(w http.ResponseWriter, r *http.Request) {
	var req taskflow.RepoReadFileReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	vmID, err := s.vmForTask(req.TaskId)
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	if registration, remote := s.remoteVM(vmID); remote {
		var result taskflow.RepoReadFile
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/task/repo-read-file", &req, &result); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, &result)
		return
	}
	path, err := s.workspacePath(vmID, req.Path)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		fail(w, statusFor(err), err)
		return
	}
	offset := int64(0)
	if req.Offset != nil && *req.Offset > 0 {
		offset = *req.Offset
	}
	if offset > int64(len(content)) {
		offset = int64(len(content))
	}
	end := int64(len(content))
	if req.Length != nil && *req.Length >= 0 && offset+*req.Length < end {
		end = offset + *req.Length
	}
	respond(w, http.StatusOK, &taskflow.RepoReadFile{TaskId: req.TaskId, RequestId: req.RequestId, Path: req.Path, Content: content[offset:end], TotalSize: int64(len(content)), Offset: offset, Length: end - offset, IsTruncated: end < int64(len(content)), Success: true})
}

func (s *Server) repoFileDiff(w http.ResponseWriter, r *http.Request) {
	var req taskflow.RepoFileDiffReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	vmID, err := s.vmForTask(req.TaskId)
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	if registration, remote := s.remoteVM(vmID); remote {
		var result taskflow.RepoFileDiff
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/task/repo-file-diff", &req, &result); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, &result)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	quoted := shellQuote(req.Path)
	output, execErr := s.docker.Exec(ctx, vmID, "git diff --no-ext-diff -- "+quoted)
	if execErr != nil && len(output) == 0 {
		message := execErr.Error()
		respond(w, http.StatusOK, &taskflow.RepoFileDiff{TaskId: req.TaskId, RequestId: req.RequestId, Path: req.Path, Success: false, Error: &message})
		return
	}
	respond(w, http.StatusOK, &taskflow.RepoFileDiff{TaskId: req.TaskId, RequestId: req.RequestId, Path: req.Path, Diff: string(output), Success: true})
}

func (s *Server) repoFileChanges(w http.ResponseWriter, r *http.Request) {
	var req taskflow.RepoFileChangesReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	vmID, err := s.vmForTask(req.TaskId)
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	if registration, remote := s.remoteVM(vmID); remote {
		var result taskflow.RepoFileChanges
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/task/repo-file-changes", &req, &result); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, &result)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	output, execErr := s.docker.Exec(ctx, vmID, "git status --porcelain=v1")
	if execErr != nil {
		message := execErr.Error()
		respond(w, http.StatusOK, &taskflow.RepoFileChanges{TaskId: req.TaskId, RequestId: req.RequestId, Success: false, Error: &message})
		return
	}
	result := &taskflow.RepoFileChanges{TaskId: req.TaskId, RequestId: req.RequestId, Success: true}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if len(line) < 4 {
			continue
		}
		result.Changes = append(result.Changes, &taskflow.RepoFileChangeInfo{Status: strings.TrimSpace(line[:2]), Path: strings.TrimSpace(line[3:])})
	}
	respond(w, http.StatusOK, result)
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
