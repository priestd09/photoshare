package controllers

import (
	"code.google.com/p/graphics-go/graphics"
	"fmt"
	"github.com/robfig/revel"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path"
	"time"
	"wharton/app/models"
)

const PHOTO_DIRECTORY = "/Users/robfig/whartonphotos"

type Application struct {
	GorpController
}

func (c Application) Index() rev.Result {
	dir, err := os.Open(PHOTO_DIRECTORY)
	if err != nil {
		return c.RenderError(fmt.Errorf("Failed to open photo directory: %s", err))
	}

	fileInfos, err := dir.Readdir(-1)
	if err != nil {
		return c.RenderError(fmt.Errorf("Failed to read photo directory: %s", err))
	}

	userPhotos := map[string][]string{}
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() && fileInfo.Name() != "thumbs" {
			userPhotos[fileInfo.Name()] = []string{}
		}
	}

	for username, _ := range userPhotos {
		userDir, err := os.Open(path.Join(PHOTO_DIRECTORY, username))
		if err != nil {
			return c.RenderError(fmt.Errorf("Failed to open user's directory: %s", err))
		}

		names, err := userDir.Readdirnames(-1)
		if err != nil {
			return c.RenderError(fmt.Errorf("Failed to read user's directory: %s", err))
		}

		userPhotos[username] = names
	}

	return c.Render(userPhotos)
}

func (c Application) Upload() rev.Result {
	return c.Render()
}

// TODO: Should be able to accept photos []*multipart.FileHeader
// TODO: Create thumbnails.  Use a native go library or imagemagick/graphicsmagick
// TODO: Support RAW (canon) and NEG (nikon)
// TODO: Handle EXIF rotation
// TODO: Read EXIF data and allow reset by time zone?
func (c Application) PostUpload(name string) rev.Result {
	c.Validation.Required(name)

	if c.Validation.HasErrors() {
		c.FlashParams()
		c.Validation.Keep()
		return c.Redirect(Application.Upload)
	}

	photoDir := path.Join(PHOTO_DIRECTORY, name)
	thumbDir := path.Join(PHOTO_DIRECTORY, "thumbs", name)
	err := os.MkdirAll(photoDir, 0777)
	if err != nil {
		c.FlashParams()
		c.Flash.Error("Error making directory:", err)
		return c.Redirect(Application.Upload)
	}
	err = os.MkdirAll(thumbDir, 0777)
	if err != nil {
		c.FlashParams()
		c.Flash.Error("Error making directory:", err)
		return c.Redirect(Application.Upload)
	}

	photos := c.Params.Files["photos[]"]
	for _, photoFileHeader := range photos {
		// Open the photo.
		input, err := photoFileHeader.Open()
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Error opening photo:", err)
			return c.Redirect(Application.Upload)
		}

		// Decode the photo.
		photoImage, format, err := image.Decode(input)
		if err != nil {
			fmt.Println("Failed to decode image:", err)
			continue
		}

		photoName := path.Base(photoFileHeader.Filename)

		// Create a thumbnail
		thumbnail := image.NewRGBA(image.Rect(0, 0, 256, 256))
		err = graphics.Thumbnail(thumbnail, photoImage)
		if err != nil {
			fmt.Println("Failed to create thumbnail:", err)
			continue
		}

		thumbnailFile, err := os.Create(path.Join(thumbDir, photoName))
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Error creating file:", err)
			return c.Redirect(Application.Upload)
		}

		err = jpeg.Encode(thumbnailFile, thumbnail, nil)
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Failed to save thumbnail:", err)
			return c.Redirect(Application.Upload)
		}

		// Save the photo
		output, err := os.Create(path.Join(photoDir, photoName))
		if err != nil {
			input.Close()
			c.FlashParams()
			c.Flash.Error("Error creating file:", err)
			return c.Redirect(Application.Upload)
		}

		_, err = io.Copy(output, input)
		input.Close()
		output.Close()
		if err != nil {
			c.FlashParams()
			c.Flash.Error("Error writing photo:", err)
			return c.Redirect(Application.Upload)
		}

		// Save a record of the photo to our database.
		rect := photoImage.Bounds()
		photo := models.Photo{
			Username: name,
			Format:   format,
			Name:     photoName,
			Width:    rect.Max.X - rect.Min.X,
			Height:   rect.Max.Y - rect.Min.Y,
			Uploaded: time.Now(),
		}

		c.Txn.Insert(&photo)
	}

	c.Flash.Success("%d photos uploaded.", len(photos))
	return c.Redirect(Application.Index)
}

func (c Application) Download(paths []string) rev.Result {
	if len(paths) == 0 {
		return c.RenderError(fmt.Errorf("Nothing to download"))
	}

	return c.Todo()
}

type PhotoServerPlugin struct {
	rev.EmptyPlugin
}

func (t PhotoServerPlugin) OnRoutesLoaded(router *rev.Router) {
	router.Routes = append([]*rev.Route{
		rev.NewRoute("GET", "/photos/", "staticDir:"+PHOTO_DIRECTORY),
	}, router.Routes...)
}

func init() {
	rev.RegisterPlugin(PhotoServerPlugin{})
}
