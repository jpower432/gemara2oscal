package evaluation

import (
	"context"
	"testing"

	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/oscal-compass/oscal-sdk-go/validation"
	"github.com/ossf/gemara/layer4"
	"github.com/stretchr/testify/require"
)

func TestToAssessmentResults(t *testing.T) {
	result := layer4.Passed
	eval := layer4.ControlEvaluation{
		Control_Id: "OSPS-QA-07",
		Assessments: []*layer4.Assessment{
			{
				Requirement_Id: "OSPS-QA-07.01",
				Message:        "Failure information",
				Methods: []layer4.AssessmentMethod{
					{
						Name:        "my-check-id",
						Description: "My method",
						Result:      &result,
					},
				},
			},
		},
	}

	plan := oscalTypes.AssessmentPlan{
		LocalDefinitions: &oscalTypes.LocalDefinitions{
			Activities: &[]oscalTypes.Activity{
				{
					UUID:  "example-uuid",
					Title: "OSPS-QA-07.01",
					Steps: &[]oscalTypes.Step{
						{
							Title: "my-check-id",
						},
					},
					RelatedControls: &oscalTypes.ReviewedControls{
						ControlSelections: []oscalTypes.AssessedControls{
							{
								IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
									{
										ControlId: "PL-8",
									},
								},
							},
						},
					},
				},
			},
		},
		Tasks: &[]oscalTypes.Task{
			{
				AssociatedActivities: &[]oscalTypes.AssociatedActivity{
					{
						ActivityUuid: "example-uuid",
					},
				},
			},
		},
		ReviewedControls: oscalTypes.ReviewedControls{
			ControlSelections: []oscalTypes.AssessedControls{
				{
					IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{
						{
							ControlId: "PL-8",
						},
					},
				},
			},
		},
	}

	ar, err := ToAssessmentResults(context.Background(), "", plan, []layer4.ControlEvaluation{eval})
	require.NoError(t, err)

	oscalModels := oscalTypes.OscalModels{
		AssessmentResults: ar,
	}
	
	validator := validation.NewSchemaValidator()
	err = validator.Validate(oscalModels)
	require.NoError(t, err)
}
