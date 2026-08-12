package httpapi

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
	"time"
)

func TestNextReportRunUsesConfiguredTimezone(t *testing.T) {
	weekday := 1
	in := reportScheduleInput{Frequency: "weekly", Timezone: "Asia/Almaty", SendHour: 9, SendWeekday: &weekday}
	now := time.Date(2026, 8, 10, 4, 0, 0, 0, time.UTC) // Monday, 09:00 in Almaty.
	next := nextReportRun(in, now)
	if want := time.Date(2026, 8, 17, 4, 0, 0, 0, time.UTC); !next.Equal(want) {
		t.Fatalf("next run = %s, want %s", next, want)
	}
}

func TestRenderXLSXCreatesReadableWorkbook(t *testing.T) {
	data, filename, mime, err := renderReport("xlsx", reportData{Company: "Dentline", Customers: 12})
	if err != nil {
		t.Fatal(err)
	}
	if filename == "" || mime != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("invalid metadata: %s %s", filename, mime)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("invalid xlsx zip: %v", err)
	}
	found := false
	for _, file := range reader.File {
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		stream, _ := file.Open()
		body, _ := io.ReadAll(stream)
		_ = stream.Close()
		found = bytes.Contains(body, []byte("Dentline"))
	}
	if !found {
		t.Fatal("worksheet does not contain report data")
	}
}

func TestRenderPDFHasValidEnvelope(t *testing.T) {
	data, _, mime, err := renderReport("pdf", reportData{Company: "Dentline"})
	if err != nil {
		t.Fatal(err)
	}
	if mime != "application/pdf" || !bytes.HasPrefix(data, []byte("%PDF-1.4")) || !bytes.Contains(data, []byte("%%EOF")) {
		t.Fatal("invalid PDF envelope")
	}
}

func TestReportInputRejectsInvalidSchedule(t *testing.T) {
	in := reportScheduleInput{Name: "Owner", Channel: "email", Recipients: []string{"owner@example.com"}, Frequency: "weekly", Format: "pdf", Timezone: "Asia/Almaty", SendHour: 9}
	if validateReportInput(&in) == "" {
		t.Fatal("weekly schedule without weekday must be rejected")
	}
}
