package storage

import "fmt"

func VideoPrefix(videoID int64) string {
	return fmt.Sprintf("videos/%d/", videoID)
}

func VideoSourceKey(videoID int64) string {
	return fmt.Sprintf("videos/%d/source", videoID)
}

func FramesPrefix(videoID int64) string {
	return fmt.Sprintf("videos/%d/frames/", videoID)
}

func FrameKey(videoID int64, idx int32) string {
	return fmt.Sprintf("videos/%d/frames/%06d.jpg", videoID, idx)
}

func EmbeddingsPrefix(videoID int64) string {
	return fmt.Sprintf("videos/%d/embeddings/", videoID)
}

// EmbeddingKey holds the SAM image embedding of a frame. The clip records
// which model computed it.
func EmbeddingKey(videoID int64, idx int32) string {
	return fmt.Sprintf("videos/%d/embeddings/%06d.pt", videoID, idx)
}

func DatasetPrefix(id int64) string {
	return fmt.Sprintf("datasets/%d/", id)
}

func DatasetKey(id int64) string {
	return fmt.Sprintf("datasets/%d/dataset.zip", id)
}

func LabelExamplesPrefix(labelTypeID int64) string {
	return fmt.Sprintf("label-types/%d/examples/", labelTypeID)
}

const BackupsPrefix = "backups/"

func BackupKey(name string) string {
	return BackupsPrefix + name
}

func TrainingRunPrefix(runID int64) string {
	return fmt.Sprintf("training-runs/%d/", runID)
}

// TrainingDatasetKey is the dataset a run trained on, in the export layout.
func TrainingDatasetKey(runID int64) string {
	return fmt.Sprintf("training-runs/%d/dataset.zip", runID)
}

// TrainingWeightsKey holds the best epoch's Ultralytics weights.
func TrainingWeightsKey(runID int64) string {
	return fmt.Sprintf("training-runs/%d/best.pt", runID)
}

// ModelUploadsPrefix holds weights the browser uploaded, until AddModel
// copies them to a run.
const ModelUploadsPrefix = "model-uploads/"

// PrelabelerPreviewKey is the mask from a pre-labeler's latest preview. Each
// preview replaces it.
func PrelabelerPreviewKey(name string) string {
	return fmt.Sprintf("prelabelers/%s/preview.png", name)
}

// DistractorsPrefix holds the distractor library's images.
const DistractorsPrefix = "distractors/"

func DistractorPrefix(id int64) string {
	return fmt.Sprintf("%s%d/", DistractorsPrefix, id)
}

// DistractorUploadKey is the image as the browser uploaded it, kept until
// processing succeeds so a failed one can be retried.
func DistractorUploadKey(id int64) string {
	return DistractorPrefix(id) + "upload"
}

func DistractorKey(id int64) string {
	return DistractorPrefix(id) + "image.jpg"
}

func DistractorThumbnailKey(id int64) string {
	return DistractorPrefix(id) + "thumb.jpg"
}
