// Package overview computes the operational KPIs of a workspace for the
// Overview page (S14): workflow executions and failures, connector health and
// webhook traffic. It only reads other modules' tables through tenant-bound
// transactions (RLS) and shows each section only to callers who can read the
// underlying resource type in the workspace.
package overview
