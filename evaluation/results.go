package evaluation

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/defenseunicorns/go-oscal/src/pkg/uuid"
	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/oscal-compass/oscal-sdk-go/extensions"
	"github.com/oscal-compass/oscal-sdk-go/transformers"
	"github.com/ossf/gemara/layer4"

	"github.com/jpower432/gemara2oscal/internal/utils"
)

// Adapted from https://github.com/oscal-compass/compliance-to-policy-go/blob/main/framework/actions/report.go

const Resource = "resource"

func ToAssessmentResults(ctx context.Context, planHref string, plan oscalTypes.AssessmentPlan, evaluations []layer4.ControlEvaluation) (*oscalTypes.AssessmentResults, error) {
	// for each PVPResult.Observation create an OSCAL Observation
	oscalObservations := make([]oscalTypes.Observation, 0)
	oscalFindings := make([]oscalTypes.Finding, 0)

	// Maps resourceIds from observation subjects to subject UUIDs
	// to avoid duplicating subjects for a single resource.
	// This is passed to toOscalObservation to maintain a global
	// state across results.
	subjectUuidMap := make(map[string]string)

	// maps resource items to subject UUIDs
	resourceItemMap := make(map[string]oscalTypes.Resource)

	// Get all the control mappings based on the assessment plan activities
	rulesByControls := make(map[string][]string)
	for _, act := range *plan.LocalDefinitions.Activities {
		var controlSet []string
		if act.RelatedControls != nil {
			controls := act.RelatedControls.ControlSelections
			for _, ctr := range controls {
				for _, assess := range *ctr.IncludeControls {
					targetId := fmt.Sprintf("%s_smt", assess.ControlId)
					controlSet = append(controlSet, targetId)
				}
			}
		}
		rulesByControls[act.Title] = controlSet
	}

	// Process into observations
	for _, evaluation := range evaluations {
		obs, err := observationsFromEvaluation(evaluation, subjectUuidMap)
		if err != nil {
			return nil, fmt.Errorf("failed to convert observation for check %v: %w", evaluation.Control_Id, err)
		}
		oscalObservations = append(oscalObservations, obs...)
	}

	assessmentResults, err := transformers.AssessmentPlanToAssessmentResults(plan, planHref, oscalObservations...)
	if err != nil {
		return nil, err
	}

	// New assessment results should only have one Assessment Results
	if len(assessmentResults.Results) != 1 {
		return nil, errors.New("bug: assessment results should only have one result")
	}

	// Create findings after initial observations are added to ensure only observations
	// in-scope of the plan are checked for failure.
	for _, obs := range *assessmentResults.Results[0].Observations {
		// TODO: Empty props indicates that an activity was in scope that results were not received for.
		// We should generate a finding here.
		if obs.Props == nil {
			continue
		}
		rule, found := extensions.GetTrestleProp(extensions.AssessmentRuleIdProp, *obs.Props)
		if !found {
			continue
		}
		targets, found := rulesByControls[rule.Value]
		if !found {
			continue
		}

		// if the observation subject result prop is not "pass" then create relevant findings
		if obs.Subjects != nil {
			for _, subject := range *obs.Subjects {

				if _, ok := resourceItemMap[subject.SubjectUuid]; !ok {
					resource := generateResource(&subject)
					resourceItemMap[subject.SubjectUuid] = resource
				}

				result, found := extensions.GetTrestleProp("result", *subject.Props)
				if !found {
					continue
				}
				if result.Value != "passed" {
					oscalFindings, err = generateFindings(oscalFindings, obs, targets)
					if err != nil {
						return nil, fmt.Errorf("failed to create finding for check: %w", err)
					}
					break
				}
			}
		}
	}

	assessmentResults.Results[0].Findings = utils.NilIfEmpty(&oscalFindings)

	if len(resourceItemMap) > 0 {
		backmatter := oscalTypes.BackMatter{}
		resources := make([]oscalTypes.Resource, 0, len(resourceItemMap))
		for _, r := range resourceItemMap {
			resources = append(resources, r)
		}
		backmatter.Resources = &resources
		assessmentResults.BackMatter = &backmatter
	}
	return assessmentResults, nil
}

// Generate an OSCAL Resource from a given Subject reference
func generateResource(subject *oscalTypes.SubjectReference) oscalTypes.Resource {
	resource := oscalTypes.Resource{
		UUID:  subject.SubjectUuid,
		Title: subject.Title,
	}
	return resource
}

// getFindingForTarget returns an existing finding that matches the targetId if one exists in findings
func getFindingForTarget(findings []oscalTypes.Finding, targetId string) *oscalTypes.Finding {
	for i := range findings {
		if findings[i].Target.TargetId == targetId {
			return &findings[i] // if finding is found, return a pointer to that slice element
		}
	}
	return nil
}

// Generate OSCAL Findings for all non-passing controls in the OSCAL Observation
func generateFindings(findings []oscalTypes.Finding, observation oscalTypes.Observation, targets []string) ([]oscalTypes.Finding, error) {
	for _, targetId := range targets {
		finding := getFindingForTarget(findings, targetId)
		if finding == nil { // if an empty finding was returned, create a new one and append to findings
			newFinding := oscalTypes.Finding{
				UUID: uuid.NewUUID(),
				RelatedObservations: &[]oscalTypes.RelatedObservation{
					{
						ObservationUuid: observation.UUID,
					},
				},
				Target: oscalTypes.FindingTarget{
					TargetId: targetId,
					Type:     "statement-id",
					Status: oscalTypes.ObjectiveStatus{
						State: "not-satisfied",
					},
				},
			}
			findings = append(findings, newFinding)
		} else {
			relObs := oscalTypes.RelatedObservation{
				ObservationUuid: observation.UUID,
			}
			*finding.RelatedObservations = append(*finding.RelatedObservations, relObs) // add new related obs to existing finding for targetId
		}
	}
	return findings, nil
}

func observationsFromEvaluation(eval layer4.ControlEvaluation, subjectUUID map[string]string) ([]oscalTypes.Observation, error) {
	var observations []oscalTypes.Observation
	for _, assessment := range eval.Assessments {
		// Metadata for raw evidence or assessment inputs would go here.
		// There is not enough in the `gemara` schema to populate this properly.
		// Should be fixed with https://github.com/revanite-io/sci/issues/23
		subjectUuid, ok := subjectUUID[assessment.Requirement_Id]
		if !ok {
			subjectUuid = uuid.NewUUID()
			subjectUUID[assessment.Requirement_Id] = subjectUuid
		}

		subj := oscalTypes.SubjectReference{
			SubjectUuid: subjectUuid,
			Title:       assessment.Message,
			Type:        Resource,
		}

		oscalObservation := oscalTypes.Observation{
			UUID:        uuid.NewUUID(),
			Title:       assessment.Requirement_Id,
			Description: assessment.Description,
			Methods:     []string{"TEST-AUTOMATED"},
			// TODO: Think this conversion more since there is no L4 timestamp
			Collected: time.Now(),
			Subjects:  &[]oscalTypes.SubjectReference{subj},
		}

		var resultString string
		switch assessment.Result {
		case layer4.Failed:
			resultString = "failed"
		case layer4.Passed:
			resultString = "passed"
		case layer4.NeedsReview:
			resultString = "needs-review"
		case layer4.NotApplicable:
			resultString = "not-applicable"
		case layer4.NotRun:
			resultString = "not-run"
		default:
			resultString = "unknown"
		}

		oscalObservation.Props = &[]oscalTypes.Property{
			{
				Name:  extensions.AssessmentRuleIdProp,
				Value: assessment.Requirement_Id,
				Ns:    extensions.TrestleNameSpace,
			},
			{
				Name:  extensions.AssessmentCheckIdProp,
				Value: assessment.Requirement_Id,
				Ns:    extensions.TrestleNameSpace,
			},
			{
				Name:  "result",
				Value: resultString,
				Ns:    extensions.TrestleNameSpace,
			},
			{
				Name:  "reason",
				Value: assessment.Message,
				Ns:    extensions.TrestleNameSpace,
			},
			{
				Name:  "steps-executed",
				Value: strconv.Itoa(assessment.Steps_Executed),
				Ns:    extensions.TrestleNameSpace,
			},
		}
		observations = append(observations, oscalObservation)
	}
	return observations, nil
}
