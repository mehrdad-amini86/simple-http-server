package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type FileInfo struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
	URL     string
}

type DirectoryListing struct {
	Path  string
	Files []FileInfo
}

var (
	port   = flag.Int("port", 8000, "Port to serve on")
	folder = flag.String("folder", "", "Folder to serve files from (required)")
)

func main() {
	flag.Parse()

	if *folder == "" {
		fmt.Println("Error: --folder is required")
		os.Exit(1)
	}

	// Validate folder path
	servePath, err := filepath.Abs(*folder)
	if err != nil {
		fmt.Printf("Error: Invalid folder path: %v\n", err)
		os.Exit(1)
	}

	// Check if folder exists
	if _, err := os.Stat(servePath); os.IsNotExist(err) {
		fmt.Printf("Error: Folder '%s' does not exist\n", servePath)
		os.Exit(1)
	}

	fmt.Printf("Serving files from: %s\n", servePath)
	fmt.Printf("Server running on: http://localhost:%d\n", *port)
	fmt.Println("Press Ctrl+C to stop the server")

	// Create HTTP handler
	handler := &FileServer{servePath: servePath}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: handler,
	}

	log.Fatal(server.ListenAndServe())
}

type FileServer struct {
	servePath string
}

func (fs *FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse the URL path
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Security check: prevent directory traversal
	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
		http.Error(w, "Forbidden: Directory traversal not allowed", http.StatusForbidden)
		return
	}

	// Build full file path
	fullPath := filepath.Join(fs.servePath, path)

	// Resolve absolute path and check it's within serve directory
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Security check: ensure path is within serve directory
	serveAbsPath, _ := filepath.Abs(fs.servePath)
	if !strings.HasPrefix(absPath, serveAbsPath) {
		http.Error(w, "Forbidden: Path outside serve directory", http.StatusForbidden)
		return
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		fs.serveDirectory(w, r, absPath, path)
	} else {
		fs.serveFile(w, r, absPath)
	}
}

func (fs *FileServer) serveFile(w http.ResponseWriter, r *http.Request, filePath string) {
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading file: %v", err), http.StatusInternalServerError)
		return
	}

	// Set headers
	filename := filepath.Base(filePath)
	w.Header().Set("Content-Type", getMimeType(filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// Copy file to response
	_, err = io.Copy(w, file)
	if err != nil {
		log.Printf("Error writing file: %v", err)
	}
}

func (fs *FileServer) serveDirectory(w http.ResponseWriter, r *http.Request, dirPath, urlPath string) {
	// Read directory contents
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading directory: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to FileInfo slice
	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileInfo := FileInfo{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}

		// Build URL
		if urlPath != "" {
			fileInfo.URL = "/" + urlPath + "/" + entry.Name()
		} else {
			fileInfo.URL = "/" + entry.Name()
		}

		// Add trailing slash for directories
		if entry.IsDir() {
			fileInfo.URL += "/"
		}

		files = append(files, fileInfo)
	}

	// Sort files: directories first, then files, both alphabetically
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	// Create directory listing
	listing := DirectoryListing{
		Path:  urlPath,
		Files: files,
	}

	// Generate HTML
	html, err := fs.generateDirectoryHTML(listing)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error generating HTML: %v", err), http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (fs *FileServer) generateDirectoryHTML(listing DirectoryListing) (string, error) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Directory listing for {{.Path}}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        a { text-decoration: none; color: #0066cc; }
        a:hover { text-decoration: underline; }
        .file-icon { color: #666; }
        .dir-icon { color: #ff6600; }
    </style>
</head>
<body>
    <h1>Directory listing for {{.Path}}</h1>
    <table>
        <thead>
            <tr>
                <th>Name</th>
                <th>Type</th>
                <th>Size</th>
                <th>Modified</th>
            </tr>
        </thead>
        <tbody>
            {{if .Path}}
            <tr>
                <td><a href="{{if eq (len (split .Path "/")) 1}}/{{else}}{{.Path | dirname}}/{{end}}">📁 ..</a></td>
                <td><span class="dir-icon">📁</span> Directory</td>
                <td>-</td>
                <td>-</td>
            </tr>
            {{end}}
            {{range .Files}}
            <tr>
                <td><a href="{{.URL}}">{{if .IsDir}}📁{{else}}📄{{end}} {{.Name}}</a></td>
                <td>{{if .IsDir}}Directory{{else}}File{{end}}</td>
                <td>{{if .IsDir}}-{{else}}{{.Size | formatBytes}}{{end}}</td>
                <td>{{.ModTime.Format "2006-01-02 15:04"}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</body>
</html>`

	// Create template with custom functions
	t, err := template.New("listing").Funcs(template.FuncMap{
		"formatBytes": func(bytes int64) string {
			if bytes < 1024 {
				return fmt.Sprintf("%d B", bytes)
			} else if bytes < 1024*1024 {
				return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
			} else {
				return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
			}
		},
		"split": strings.Split,
		"dirname": func(path string) string {
			parts := strings.Split(path, "/")
			if len(parts) <= 1 {
				return ""
			}
			return strings.Join(parts[:len(parts)-1], "/")
		},
	}).Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	err = t.Execute(&buf, listing)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func getMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	default:
		return "application/octet-stream"
	}
}
