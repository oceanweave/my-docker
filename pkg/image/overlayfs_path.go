package image

import "fmt"

const (
	ImagePath         = "/var/lib/mydocker/image/"
	OverlayfsRootPath = "/var/lib/mydocker/overlay2/"
	lowerDirFormat    = OverlayfsRootPath + "%s/lower"
	upperDirFormat    = OverlayfsRootPath + "%s/upper"
	workDirFormat     = OverlayfsRootPath + "%s/work"
	mergedDirFormat   = OverlayfsRootPath + "%s/merged"
	overlayFSFormat   = "lowerdir=%s,upperdir=%s,workdir=%s"
)

func GetRootDir(containerId string) string {
	return OverlayfsRootPath + containerId
}

func GetImageDir(imageName string) string {
	return fmt.Sprintf("%s%s.tar", ImagePath, imageName)
}

func GetLowerDir(containerId string) string {
	return fmt.Sprintf(lowerDirFormat, containerId)
}

func GetUpperDir(containerId string) string {
	return fmt.Sprintf(upperDirFormat, containerId)
}

func GetWorkDir(containerId string) string {
	return fmt.Sprintf(workDirFormat, containerId)
}

func GetMergedDir(containerId string) string {
	return fmt.Sprintf(mergedDirFormat, containerId)
}

func GetOverlayFSDirs(lower, upper, work string) string {
	return fmt.Sprintf(overlayFSFormat, lower, upper, work)
}
