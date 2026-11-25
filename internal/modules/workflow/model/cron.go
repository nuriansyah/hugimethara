package model

import platformcron "github.com/nuriansyah/hugimethara/internal/platform/cron"

// Cron is the platform cron type, re-exported for workflow configs.
type Cron = platformcron.Cron

// ParseCron parses a five-field expression.
func ParseCron(expr string) (Cron, error) { return platformcron.Parse(expr) }
