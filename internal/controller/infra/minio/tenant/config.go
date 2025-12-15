package tenant

import (
	"fmt"
	"regexp"
)

type MinioEnvConfig struct {
	RootUser            string
	MinioBrowserSetting string
}

type minioConfigFile struct {
	rootUser            string
	rootPassword        string
	minioBrowserSetting string
}

func buildMinioConfigFile(
	rootUser, rootPassword, minioBrowserSetting string,
) minioConfigFile {
	return minioConfigFile{
		rootUser:            rootUser,
		rootPassword:        rootPassword,
		minioBrowserSetting: minioBrowserSetting,
	}
}

func parseMinioConfigFile(fileContents string) minioConfigFile {
	rootUserRegex := regexp.MustCompile(`export MINIO_ROOT_USER="([^"]*)"`)
	rootPasswordRegex := regexp.MustCompile(`export MINIO_ROOT_PASSWORD="([^"]*)"`)
	browserSettingRegex := regexp.MustCompile(`export MINIO_BROWSER="([^"]*)"`)

	var config minioConfigFile

	if matches := rootUserRegex.FindStringSubmatch(fileContents); len(matches) > 1 {
		config.rootUser = matches[1]
	}

	if matches := rootPasswordRegex.FindStringSubmatch(fileContents); len(matches) > 1 {
		config.rootPassword = matches[1]
	}

	if matches := browserSettingRegex.FindStringSubmatch(fileContents); len(matches) > 1 {
		config.minioBrowserSetting = matches[1]
	}

	return config
}

func (m minioConfigFile) toFileContents() string {
	return fmt.Sprintf(`export MINIO_ROOT_USER="%s"
	export MINIO_ROOT_PASSWORD="%s"
export MINIO_BROWSER="%s"`, m.rootUser, m.rootPassword, m.minioBrowserSetting)
}
