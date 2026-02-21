# Simple HTTP Server

A simple HTTP server with directory listing and file serving capabilities.

## Features

- Serve files from any directory
- Directory listing with HTML interface
- Recursive folder support
- Security protection against directory traversal
- Simple command-line interface

## Usage

```bash
# Make the server executable (if not already)
chmod +x server

# Run the server
./server --port 1717 --folder ./files/

# Or with absolute path
./server --port 1717 --folder /home/debian/files/
```

## Examples

If you have a file at `./files/test/sub/2.png`, you can access it via:
- `http://localhost:1717/test/sub/2.png`

If you want to browse a directory, go to:
- `http://localhost:1717/test/sub/` - Shows files in the subdirectory
- `http://localhost:1717/` - Shows files in the root directory

## Security Features

- Prevents directory traversal attacks (no `../` allowed)
- Validates that requested files are within the serve directory
- Proper MIME type detection for file downloads

## Requirements

- Python 3.6+
- No external dependencies (uses only standard library)
