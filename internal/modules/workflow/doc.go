// Package workflow is the native workflow engine bounded context (S10,
// ADR-0003, docs/architecture/workflow-runtime.md).
//
//	model/       definition, node catalog, validator, step/execution state machines, graph progression (pure)
//	model/dto/   use-case inputs
//	repository/  typed persistence functions; SQL split into commands/ and queries/
//	services/    WorkflowService (control plane, hugi_app) and Engine (data plane, hugi_system)
package workflow
