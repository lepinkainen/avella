package template

import "strings"

// File type categories.
const (
	TypeVideo    = "Video"
	TypeImage    = "Image"
	TypeAudio    = "Audio"
	TypeDocument = "Document"
	TypeArchive  = "Archive"
	TypeOther    = "Other"
)

var extTypes = map[string]string{
	// Video
	"mkv":  TypeVideo,
	"mp4":  TypeVideo,
	"avi":  TypeVideo,
	"mov":  TypeVideo,
	"wmv":  TypeVideo,
	"flv":  TypeVideo,
	"webm": TypeVideo,
	"m4v":  TypeVideo,
	"ts":   TypeVideo,
	"mpg":  TypeVideo,
	"mpeg": TypeVideo,

	// Image
	"jpg":  TypeImage,
	"jpeg": TypeImage,
	"png":  TypeImage,
	"gif":  TypeImage,
	"bmp":  TypeImage,
	"webp": TypeImage,
	"svg":  TypeImage,
	"tiff": TypeImage,
	"tif":  TypeImage,
	"heic": TypeImage,
	"heif": TypeImage,
	"avif": TypeImage,
	"raw":  TypeImage,

	// Audio
	"mp3":  TypeAudio,
	"flac": TypeAudio,
	"wav":  TypeAudio,
	"aac":  TypeAudio,
	"ogg":  TypeAudio,
	"wma":  TypeAudio,
	"m4a":  TypeAudio,
	"opus": TypeAudio,

	// Document
	"pdf":  TypeDocument,
	"doc":  TypeDocument,
	"docx": TypeDocument,
	"xls":  TypeDocument,
	"xlsx": TypeDocument,
	"ppt":  TypeDocument,
	"pptx": TypeDocument,
	"txt":  TypeDocument,
	"rtf":  TypeDocument,
	"csv":  TypeDocument,
	"odt":  TypeDocument,

	// Archive
	"zip": TypeArchive,
	"tar": TypeArchive,
	"gz":  TypeArchive,
	"bz2": TypeArchive,
	"xz":  TypeArchive,
	"7z":  TypeArchive,
	"rar": TypeArchive,
	"zst": TypeArchive,
}

// ClassifyExt returns the file type category for a given extension (without dot).
// Returns "Other" for unknown extensions.
func ClassifyExt(ext string) string {
	if t, ok := extTypes[strings.ToLower(ext)]; ok {
		return t
	}
	return TypeOther
}
