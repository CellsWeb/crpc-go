package imagehandler

import (
	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"example-mongo/utils/storage"
)

func ProcessImageField(file *rpc.UploadedFile, dir string, existingURL string, engine ...*storage.Engine) (string, error) {
	eng := storage.DefaultEngine
	if len(engine) > 0 && engine[0] != nil {
		eng = engine[0]
	}
	if dir == "" {
		dir = "general"
	}

	if file != nil {
		data, err := file.Bytes()
		if err != nil {
			return "", err
		}
		res, err := eng.Upload(data, file.Filename(), dir)
		if err != nil {
			return "", err
		}
		if existingURL != "" {
			_ = eng.Delete(existingURL)
		}
		return eng.ToRelativeImageUrl(res.URL), nil
	}

	if existingURL != "" {
		return eng.ToRelativeImageUrl(existingURL), nil
	}
	return "", nil
}

func ProcessMultipleImageFields(files []*rpc.UploadedFile, dir string, removeList []string, existingList []string, engine ...*storage.Engine) ([]string, error) {
	eng := storage.DefaultEngine
	if len(engine) > 0 && engine[0] != nil {
		eng = engine[0]
	}
	if dir == "" {
		dir = "gallery"
	}

	for _, toRemove := range removeList {
		if toRemove != "" {
			_ = eng.Delete(toRemove)
		}
	}

	var finalURLs []string
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
