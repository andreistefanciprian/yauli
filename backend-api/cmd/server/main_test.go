package main

import (
	"strings"
	"testing"
)

func TestRunCommandRejectsMissingCommand(t *testing.T) {
	if err := runCommand(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunCommandRejectsUnknownCommand(t *testing.T) {
	err := runCommand([]string{"nope"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error = %v, want unknown command", err)
	}
}

func TestRunCommandRejectsExtraArgs(t *testing.T) {
	err := runCommand([]string{sendDailyReportEmailsCommand, "extra"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("error = %v, want usage", err)
	}
}

func TestSendDailyReportEmailsCommandName(t *testing.T) {
	if sendDailyReportEmailsCommand != "send-daily-report-emails" {
		t.Fatalf("command = %q", sendDailyReportEmailsCommand)
	}
}
