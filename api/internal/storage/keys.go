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
