// Package imagebuild runs live-build to produce the NuDanOS ISO (spec 7.1 image).
package imagebuild

import (
	"strconv"

	"github.com/nudanos/distro/internal/engine"
)

// Spec builds the ISO from the signed repo in repoDir with the live-build
// config in configDir, writing nudanos-<version>-amd64.iso to outDir. The
// script runs from the config mount, so changing it does not change the
// builder image (part of every package's cache key).
func Spec(image, repoDir, configDir, outDir, version, epoch string, uid, gid int) engine.RunSpec {
	return engine.RunSpec{Image: image, Privileged: true,
		Mounts: []engine.Mount{
			{Host: repoDir, Container: "/repo", ReadOnly: true},
			{Host: configDir, Container: "/image", ReadOnly: true},
			{Host: outDir, Container: "/out"},
		},
		Env: map[string]string{"NUDANOS_VERSION": version, "SOURCE_DATE_EPOCH": epoch,
			"HOST_UID": strconv.Itoa(uid), "HOST_GID": strconv.Itoa(gid)},
		Cmd: []string{"bash", "/image/make-image.sh"}}
}
