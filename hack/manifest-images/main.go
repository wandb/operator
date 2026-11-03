// Command manifest-images prints the images a published W&B server manifest deploys, one reference per line, so the
// nightly scan covers exactly what customers run. References are built with ImageRef.GetImage, as the operator does.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
	"oras.land/oras-go/v2/registry/remote"
)

// releaseTag matches published releases only; the repository also holds nightly and branch builds.
var releaseTag = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func main() {
	repository := flag.String("repository", apiv2.DefaultManifestRepository, "server manifest repository")
	version := flag.String("version", "", "server version to scan (default: the newest release tag)")
	includeWandb := flag.Bool("include-wandb", false, "also print W&B's own application images")
	showVersion := flag.Bool("show-version", false, "print the resolved server version and exit")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	v := *version
	if v == "" {
		var err error
		if v, err = newestRelease(ctx, *repository); err != nil {
			fail(err)
		}
	}
	if *showVersion {
		fmt.Println(v)
		return
	}
	fmt.Fprintf(os.Stderr, "scanning server manifest %s from %s\n", v, *repository)

	mfst, err := manifest.GetServerManifest(ctx, *repository, v, nil)
	if err != nil {
		fail(fmt.Errorf("load manifest %s: %w", v, err))
	}
	for _, ref := range images(mfst, *includeWandb) {
		fmt.Println(ref)
	}
}

func newestRelease(ctx context.Context, repository string) (string, error) {
	repo, err := remote.NewRepository(strings.TrimPrefix(repository, "oci://"))
	if err != nil {
		return "", err
	}
	var newest *semver.Version
	err = repo.Tags(ctx, "", func(tags []string) error {
		for _, t := range tags {
			if !releaseTag.MatchString(t) {
				continue
			}
			if v := semver.MustParse(t); newest == nil || v.GreaterThan(newest) {
				newest = v
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("list tags of %s: %w", repository, err)
	}
	if newest == nil {
		return "", fmt.Errorf("no release tags in %s", repository)
	}
	return newest.Original(), nil
}

// images collects every image the manifest deploys: infra (Bucket, ClickHouse, Keeper, Kafka, MySQL, Redis) and,
// optionally, the applications, their containers and the migration jobs.
func images(m manifest.Manifest, includeWandb bool) []string {
	seen := map[string]bool{}
	add := func(img manifest.ImageRef) {
		if img.Repository != "" {
			seen[img.GetImage("")] = true
		}
	}
	for _, infra := range []map[string]manifest.InfraConfig{m.Bucket, m.Clickhouse, m.ClickhouseKeeper, m.Mysql, m.Redis} {
		for _, cfg := range infra {
			for _, img := range cfg.Images {
				add(img)
			}
		}
	}
	for _, img := range m.Kafka.Images {
		add(img)
	}
	if includeWandb {
		for _, app := range m.Applications {
			add(app.Image)
			for _, c := range append(app.InitContainers, app.Containers...) {
				add(c.Image)
			}
		}
		for _, job := range m.Migrations {
			add(job.Image)
		}
	}
	out := make([]string, 0, len(seen))
	for ref := range seen {
		out = append(out, ref)
	}
	sort.Strings(out)
	return out
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "manifest-images:", err)
	os.Exit(1)
}
