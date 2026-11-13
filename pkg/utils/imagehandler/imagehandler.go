package imagehandler

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/utils/storage"
)

// ProcessImageField handles uploading a new file and cleaning up replaced images
func ProcessImageField(file *rpc.UploadedFile, dir string, existingURL string, engine ...*storage.Engine) (string, error) {
	eng := storage.DefaultEngine
	if len(engine) > 0 && engine[0] != nil {
		eng = engine[0]
	}

	if dir == "" {
		dir = "general"
	}

	// 1. If an uploaded file is passed, store it
	if file != nil {
		data, err := file.Bytes()
		if err != nil {
			return "", err
		}

		res, err := eng.Upload(data, file.Filename(), dir)
		if err != nil {
			return "", err
		}

		// Delete previously existing file if being replaced
		if existingURL != "" {
			_ = eng.Delete(existingURL)
		}

		return eng.ToRelativeImageUrl(res.URL), nil
	}

	// 2. No new file; retain existing URL
	if existingURL != "" {
		return eng.ToRelativeImageUrl(existingURL), nil
	}

	return "", nil
}

// ProcessMultipleImageFields handles batch uploading and deleting removed images
func ProcessMultipleImageFields(files []*rpc.UploadedFile, dir string, removeList []string, existingList []string, engine ...*storage.Engine) ([]string, error) {
	eng := storage.DefaultEngine
	if len(engine) > 0 && engine[0] != nil {
		eng = engine[0]
	}

	if dir == "" {
		dir = "gallery"
	}

	// 1. Delete files specified in removeList
	for _, toRemove := range removeList {
		if toRemove != "" {
			_ = eng.Delete(toRemove)
		}
	}

	var finalURLs []string

	// 2. Retain existing items that are not in removeList
	removeSet := make(map[string]bool)
	for _, r := range removeList {
		removeSet[eng.ToRelativeImageUrl(r)] = true
	}

	for _, ex := range existingList {
		rel := eng.ToRelativeImageUrl(ex)
		if !removeSet[rel] && rel != "" {
			finalURLs = append(finalURLs, rel)
		}
	}

	// 3. Upload all new incoming files
	for _, file := range files {
		if file != nil {
			data, err := file.Bytes()
			if err != nil {
				return nil, err
			}

			res, err := eng.Upload(data, file.Filename(), dir)
			if err != nil {
				return nil, err
			}
			finalURLs = append(finalURLs, eng.ToRelativeImageUrl(res.URL))
		}
	}

	return finalURLs, nil
}
