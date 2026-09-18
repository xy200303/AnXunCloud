package service

import "testing"

// TestValidItemExceptionType 逃生类型白名单：camera_broken（相机故障）新增；label_missing 仅抽查合成项（此处仅判合法性）。
func TestValidItemExceptionType(t *testing.T) {
	for _, et := range []string{"device_missing", "unable_to_capture", "camera_broken", "label_missing"} {
		if !validItemExceptionType(et) {
			t.Errorf("%s 应为合法逃生类型", et)
		}
	}
	for _, et := range []string{"", "other", "DEVICE_MISSING"} {
		if validItemExceptionType(et) {
			t.Errorf("%q 不应为合法逃生类型", et)
		}
	}
}

// TestEscapePhotoExempt 逃生佐证分流：unable_to_capture/camera_broken 免佐证（无法拍摄还要照片是矛盾的）；
// device_missing（现场佐证）与 label_missing（最后一张标签照）仍须照片。
func TestEscapePhotoExempt(t *testing.T) {
	for _, et := range []string{"unable_to_capture", "camera_broken"} {
		if !escapePhotoExempt(et) {
			t.Errorf("%s 应免佐证照片", et)
		}
	}
	for _, et := range []string{"device_missing", "label_missing", ""} {
		if escapePhotoExempt(et) {
			t.Errorf("%q 不应免佐证照片", et)
		}
	}
}
