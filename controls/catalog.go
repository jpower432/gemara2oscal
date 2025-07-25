package controls

import (
	"fmt"
	"strings"
	"time"

	"github.com/defenseunicorns/go-oscal/src/pkg/uuid"
	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	"github.com/oscal-compass/oscal-sdk-go/extensions"
	"github.com/oscal-compass/oscal-sdk-go/models"
	"github.com/ossf/gemara/layer1"

	"github.com/jpower432/gemara2oscal/internal/utils"
)

// TODO: Add ToResolvedCatalog where the shared guidelines are fully resolved

func ToCatalog(guidance layer1.GuidanceDocument) (oscalTypes.Catalog, error) {
	metadata := models.NewSampleMetadata()
	metadata.Title = guidance.Metadata.Title

	published, err := time.Parse(time.DateOnly, guidance.Metadata.PublicationDate)
	if err != nil {
		return oscalTypes.Catalog{}, err
	}
	metadata.Published = &published

	lastModified, err := time.Parse(time.DateTime, guidance.Metadata.LastModified)
	if err != nil {
		return oscalTypes.Catalog{}, err
	}

	metadata.LastModified = lastModified
	metadata.Version = guidance.Metadata.Version

	authorRole := oscalTypes.Role{
		ID:          "author",
		Description: "Author of the guidance document",
		Title:       "Author",
	}

	author := oscalTypes.Party{
		UUID: uuid.NewUUID(),
		Type: "person",
		Name: guidance.Metadata.Author,
	}

	responsibleParty := oscalTypes.ResponsibleParty{
		PartyUuids: []string{author.UUID},
		RoleId:     authorRole.ID,
	}

	metadata.Parties = &[]oscalTypes.Party{author}
	metadata.Roles = &[]oscalTypes.Role{authorRole}
	metadata.ResponsibleParties = &[]oscalTypes.ResponsibleParty{responsibleParty}

	// Create a resource map for control linking
	resourcesMap := make(map[string]string)
	backmatter := resourcesToBackMatter(guidance.Metadata.Resources)
	if backmatter != nil {
		for _, resource := range *backmatter.Resources {
			// Extract the id from the props
			props := *resource.Props
			id := props[0].Value
			resourcesMap[id] = resource.UUID
		}
	}

	var groups []oscalTypes.Group
	for _, category := range guidance.Categories {
		groups = append(groups, createControlGroup(category, resourcesMap))
	}

	catalog := oscalTypes.Catalog{
		UUID:       uuid.NewUUID(),
		Metadata:   metadata,
		Groups:     utils.NilIfEmpty(&groups),
		BackMatter: backmatter,
	}
	return catalog, nil
}

func createControlGroup(category layer1.Category, resourcesMap map[string]string) oscalTypes.Group {
	group := oscalTypes.Group{
		ID:    category.Id,
		Title: category.Title,
	}

	controlMap := make(map[string]oscalTypes.Control)
	for _, guideline := range category.Guidelines {
		control, parent := guidelineToControl(guideline, resourcesMap)

		if parent == "" {
			controlMap[control.ID] = control
		} else {
			parentControl := controlMap[parent]
			if parentControl.Controls == nil {
				parentControl.Controls = &[]oscalTypes.Control{}
			}
			*parentControl.Controls = append(*parentControl.Controls, control)
			controlMap[parent] = parentControl
		}
	}

	controls := make([]oscalTypes.Control, 0, len(controlMap))
	for _, control := range controlMap {
		controls = append(controls, control)
	}

	group.Controls = utils.NilIfEmpty(&controls)
	return group
}

func resourcesToBackMatter(resourceRefs []layer1.ResourceReference) *oscalTypes.BackMatter {
	var resources []oscalTypes.Resource
	for _, ref := range resourceRefs {
		resource := oscalTypes.Resource{
			UUID:        uuid.NewUUID(),
			Title:       ref.Title,
			Description: ref.Description,
			Props: &[]oscalTypes.Property{
				{
					Name:  "id",
					Value: ref.Id,
					Ns:    extensions.TrestleNameSpace,
				},
			},
			Rlinks: &[]oscalTypes.ResourceLink{
				{
					Href: ref.Url,
				},
			},
			Citation: &oscalTypes.Citation{
				Text: fmt.Sprintf(
					"%s. (%s). *%s*. %s",
					ref.IssuingBody,
					ref.PublicationDate,
					ref.Title,
					ref.Url),
			},
		}
		resources = append(resources, resource)
	}

	if len(resources) == 0 {
		return nil
	}

	backmatter := oscalTypes.BackMatter{
		Resources: &resources,
	}
	return &backmatter
}

func guidelineToControl(guideline layer1.Guideline, resourcesMap map[string]string) (oscalTypes.Control, string) {
	control := oscalTypes.Control{
		ID:    guideline.Id,
		Title: guideline.Title,
	}

	var links []oscalTypes.Link
	for _, also := range guideline.SeeAlso {
		relatedLink := oscalTypes.Link{
			Href: fmt.Sprintf("#%s", also),
			Rel:  "related",
		}
		links = append(links, relatedLink)
	}

	for _, external := range guideline.ExternalReferences {
		ref, found := resourcesMap[external]
		if !found {
			continue
		}
		externalLink := oscalTypes.Link{
			Href: fmt.Sprintf("#%s", ref),
			Rel:  "reference",
		}
		links = append(links, externalLink)
	}

	// objective part
	objPart := oscalTypes.Part{
		Name:  "assessment-objective",
		ID:    fmt.Sprintf("%s_obj", guideline.Id),
		Prose: guideline.Objective,
	}

	// top level smt
	smtPart := oscalTypes.Part{
		Name: "statement",
		ID:   fmt.Sprintf("%s_smt", guideline.Id),
	}
	var subSmts []oscalTypes.Part
	for _, part := range guideline.GuidelineParts {
		subSmt := oscalTypes.Part{
			ID:    fmt.Sprintf("%s_smt.%s", guideline.Id, part.Id),
			Prose: part.Prose,
			Title: part.Title,
		}

		if len(part.Recommendations) > 0 {
			gdnSubPart := oscalTypes.Part{
				Name:  "guidance",
				ID:    fmt.Sprintf("%s_smt.%s_gdn", guideline.Id, part.Id),
				Prose: strings.Join(part.Recommendations, " "),
			}
			subSmt.Parts = &[]oscalTypes.Part{
				gdnSubPart,
			}
		}

		subSmts = append(subSmts, subSmt)
	}
	smtPart.Parts = utils.NilIfEmpty(&subSmts)

	control.Parts = &[]oscalTypes.Part{
		objPart,
		smtPart,
	}

	if len(guideline.Recommendations) > 0 {
		// gdn part
		gdnPart := oscalTypes.Part{
			Name:  "guidance",
			ID:    fmt.Sprintf("%s_gdn", guideline.Id),
			Prose: strings.Join(guideline.Recommendations, " "),
		}
		*control.Parts = append(*control.Parts, gdnPart)
	}

	return control, guideline.BaseGuidelineID
}
