package tests

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"tests/util"
)

// assertProjectExportRatings checks both download formats against live judging totals.
func assertProjectExportRatings(context *util.Context, expected map[string]map[string]string) util.Result {
	check := func(body, challenge string) error {
		rows, err := csv.NewReader(strings.NewReader(body)).ReadAll()
		if err != nil {
			return err
		}
		if len(rows) < 2 {
			return fmt.Errorf("export has no projects")
		}
		for _, row := range rows[1:] {
			values := make(map[string]string)
			for i, column := range rows[0] {
				values[column] = row[i]
			}
			want, ok := expected[values["Name"]]
			if !ok {
				return fmt.Errorf("unexpected exported project %q", values["Name"])
			}
			for column, value := range want {
				actual, present := values[column]
				if !present || actual != value {
					return fmt.Errorf("%s: %s=%q, want %q", values["Name"], column, actual, value)
				}
			}
			if challenge != "" {
				entered := false
				for _, name := range strings.Split(values["ChallengeList"], ",") {
					if name == challenge {
						entered = true
					}
				}
				if !entered {
					return fmt.Errorf("project outside %s included in its export", challenge)
				}
			}
		}
		if challenge == "" && len(rows)-1 != len(expected) {
			return fmt.Errorf("project export is missing rows")
		}
		return nil
	}
	for _, path := range []string{"/admin/export/projects", "/admin/export/challenges"} {
		status, _ := util.GetRequestWithStatus(context.Logger, path, util.DefaultAuth())
		if status != 401 && status != 403 {
			return util.NewResult(false, "Export must require admin authentication")
		}
	}
	status, csvBody := util.GetRequestWithStatus(context.Logger, "/admin/export/projects", util.AdminAuth())
	if status != 200 {
		return util.NewResult(false, "Project export failed")
	}
	if err := check(csvBody, ""); err != nil {
		return util.NewResult(false, err.Error())
	}
	status, zipBody := util.GetRequestWithStatus(context.Logger, "/admin/export/challenges", util.AdminAuth())
	if status != 200 {
		return util.NewResult(false, "Challenge export failed")
	}
	archive, err := zip.NewReader(bytes.NewReader([]byte(zipBody)), int64(len(zipBody)))
	if err != nil {
		return util.NewResult(false, err.Error())
	}
	if len(archive.File) == 0 {
		return util.NewResult(false, "Challenge export contains no files")
	}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			return util.NewResult(false, err.Error())
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return util.NewResult(false, err.Error())
		}
		if err := check(string(data), strings.TrimSuffix(file.Name, ".csv")); err != nil {
			return util.NewResult(false, err.Error())
		}
	}
	return util.ResultOk()
}
