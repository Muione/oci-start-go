// Package oci -- ai_vision.go: Phase 14.3 OCI AI Vision SDK wrapper.
// Provides image analysis (classification, object detection, text recognition),
// document analysis, and async video analysis via the OCI AI Vision service.
package oci

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/aivision"
	"github.com/oracle/oci-go-sdk/v65/common"
)

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// AnalyzeImageInput is the service-layer input for image analysis.
type AnalyzeImageInput struct {
	CompartmentID string   `json:"compartmentId"`
	ImageData     string   `json:" imageData"` // base64-encoded image data
	NamespaceName string   `json:"namespaceName"`
	BucketName    string   `json:"bucketName"`
	ObjectName    string   `json:"objectName"`
	Features      []string `json:"features"` // CLASSIFICATION, OBJECT_DETECTION, TEXT_RECOGNITION
	MaxResults    int      `json:"maxResults"`
}

// AnalyzeImageOutput is the API response for image analysis.
type AnalyzeImageOutput struct {
	ImageClassificationLabels []ClassificationLabel `json:"imageClassificationLabels,omitempty"`
	DetectedObjects           []DetectedObject      `json:"detectedObjects,omitempty"`
	DetectedText              []DetectedTextLine    `json:"detectedText,omitempty"`
}

// ClassificationLabel is a classification result.
type ClassificationLabel struct {
	Name       string  `json:"name"`
	Confidence float32 `json:"confidence"`
}

// DetectedObject is an object detection result.
type DetectedObject struct {
	Name            string           `json:"name"`
	Confidence      float32          `json:"confidence"`
	BoundingPolygon *PolygonVO       `json:"boundingPolygon,omitempty"`
	Labels          []ClassificationLabel `json:"labels,omitempty"`
}

// DetectedTextLine is a text detection result.
type DetectedTextLine struct {
	Text            string           `json:"text"`
	Confidence      float32          `json:"confidence"`
	BoundingPolygon *PolygonVO       `json:"boundingPolygon,omitempty"`
	Words           []DetectedWord   `json:"words,omitempty"`
}

// DetectedWord is a single word detection.
type DetectedWord struct {
	Text       string  `json:"text"`
	Confidence float32 `json:"confidence"`
}

// PolygonVO is a bounding polygon with normalized vertices.
type PolygonVO struct {
	NormalizedVertices []VertexVO `json:"normalizedVertices"`
}

// VertexVO is a normalized (x, y) coordinate.
type VertexVO struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// AnalyzeDocumentInput is the service-layer input for document analysis.
type AnalyzeDocumentInput struct {
	CompartmentID string   `json:"compartmentId"`
	NamespaceName string   `json:"namespaceName"`
	BucketName    string   `json:"bucketName"`
	ObjectName    string   `json:"objectName"`
	Features      []string `json:"features"` // TABLE_DETECTION, KEY_VALUE_EXTRACTION, LANGUAGE_CLASSIFICATION
	MaxResults    int      `json:"maxResults"`
}

// AnalyzeDocumentOutput is the API response for document analysis.
type AnalyzeDocumentOutput struct {
	Pages []DocumentPageVO `json:"pages,omitempty"`
}

// DocumentPageVO is a page in the document analysis result.
type DocumentPageVO struct {
	PageNumber int                    `json:"pageNumber"`
	Width      float32                `json:"width,omitempty"`
	Height     float32                `json:"height,omitempty"`
	Tables     []DocumentTableVO      `json:"tables,omitempty"`
	KeyValuePairs []KeyValuePairVO     `json:"keyValuePairs,omitempty"`
	Languages  []DocumentLanguageVO   `json:"languages,omitempty"`
}

// DocumentTableVO is a detected table.
type DocumentTableVO struct {
	RowCount    int            `json:"rowCount"`
	ColumnCount int            `json:"columnCount"`
	Cells       []TableCellVO  `json:"cells,omitempty"`
}

// TableCellVO is a cell in a detected table.
type TableCellVO struct {
	Text        string  `json:"text"`
	RowIndex    int     `json:"rowIndex"`
	ColumnIndex int     `json:"columnIndex"`
	Confidence  float32 `json:"confidence,omitempty"`
}

// KeyValuePairVO is a detected key-value pair.
type KeyValuePairVO struct {
	Key   FieldVO `json:"key"`
	Value FieldVO `json:"value"`
}

// FieldVO is a field in a key-value pair.
type FieldVO struct {
	Text       string  `json:"text"`
	Confidence float32 `json:"confidence,omitempty"`
}

// DocumentLanguageVO is a detected language.
type DocumentLanguageVO struct {
	Language   string  `json:"language"`
	Confidence float32 `json:"confidence"`
}

// AnalyzeVideoInput is the service-layer input for video analysis.
type AnalyzeVideoInput struct {
	CompartmentID      string `json:"compartmentId"`
	NamespaceName      string `json:"namespaceName"`
	BucketName         string `json:"bucketName"`
	ObjectName         string `json:"objectName"`
	Features           []string `json:"features"` // OBJECT_TRACKING, LABEL_DETECTION
	OutputNamespace    string `json:"outputNamespaceName"`
	OutputBucket       string `json:"outputBucketName"`
	OutputPrefix       string `json:"outputPrefix"`
	DisplayName        string `json:"displayName"`
}

// AnalyzeVideoOutput is the API response for starting video analysis.
type AnalyzeVideoOutput struct {
	WorkRequestID string `json:"workRequestId"`
}

// VideoAnalysisStatus is the status of a video analysis job.
type VideoAnalysisStatus struct {
	WorkRequestID  string  `json:"workRequestId"`
	Status         string  `json:"status"`
	PercentComplete float32 `json:"percentComplete"`
	TimeAccepted   string  `json:"timeAccepted,omitempty"`
	TimeStarted    string  `json:"timeStarted,omitempty"`
	TimeFinished   string  `json:"timeFinished,omitempty"`
	DisplayName    string  `json:"displayName,omitempty"`
}

// ---------------------------------------------------------------------------
// SDK wrappers
// ---------------------------------------------------------------------------

// AnalyzeImage analyzes an image (inline base64 or Object Storage reference).
func AnalyzeImage(ctx context.Context, c *aivision.AIServiceVisionClient, in AnalyzeImageInput) (*AnalyzeImageOutput, error) {
	details := aivision.AnalyzeImageDetails{
		CompartmentId: common.String(in.CompartmentID),
	}

	// Set image source.
	if in.ImageData != "" {
		decoded, err := base64.StdEncoding.DecodeString(in.ImageData)
		if err != nil {
			return nil, fmt.Errorf("aivision: decode base64 image: %w", err)
		}
		details.Image = aivision.InlineImageDetails{Data: decoded}
	} else if in.NamespaceName != "" && in.BucketName != "" && in.ObjectName != "" {
		details.Image = aivision.ObjectStorageImageDetails{
			NamespaceName: common.String(in.NamespaceName),
			BucketName:    common.String(in.BucketName),
			ObjectName:    common.String(in.ObjectName),
		}
	} else {
		return nil, fmt.Errorf("aivision: either imageData or object storage location required")
	}

	// Build features.
	features, err := buildImageFeatures(in.Features, in.MaxResults)
	if err != nil {
		return nil, err
	}
	details.Features = features

	resp, err := c.AnalyzeImage(ctx, aivision.AnalyzeImageRequest{
		AnalyzeImageDetails: details,
	})
	if err != nil {
		return nil, fmt.Errorf("aivision: analyze image: %w", err)
	}

	return buildAnalyzeImageOutput(resp.AnalyzeImageResult), nil
}

// AnalyzeDocument analyzes a document from Object Storage.
func AnalyzeDocument(ctx context.Context, c *aivision.AIServiceVisionClient, in AnalyzeDocumentInput) (*AnalyzeDocumentOutput, error) {
	if in.NamespaceName == "" || in.BucketName == "" || in.ObjectName == "" {
		return nil, fmt.Errorf("aivision: document analysis requires object storage location")
	}

	details := aivision.AnalyzeDocumentDetails{
		CompartmentId: common.String(in.CompartmentID),
		Document: aivision.ObjectStorageDocumentDetails{
			NamespaceName: common.String(in.NamespaceName),
			BucketName:    common.String(in.BucketName),
			ObjectName:    common.String(in.ObjectName),
		},
	}

	// Build document features.
	features, err := buildDocumentFeatures(in.Features)
	if err != nil {
		return nil, err
	}
	details.Features = features

	resp, err := c.AnalyzeDocument(ctx, aivision.AnalyzeDocumentRequest{
		AnalyzeDocumentDetails: details,
	})
	if err != nil {
		return nil, fmt.Errorf("aivision: analyze document: %w", err)
	}

	return buildAnalyzeDocumentOutput(resp.AnalyzeDocumentResult), nil
}

// AnalyzeVideo starts an async video analysis job.
func AnalyzeVideo(ctx context.Context, c *aivision.AIServiceVisionClient, in AnalyzeVideoInput) (*AnalyzeVideoOutput, error) {
	if in.NamespaceName == "" || in.BucketName == "" || in.ObjectName == "" {
		return nil, fmt.Errorf("aivision: video analysis requires input object storage location")
	}
	if in.OutputNamespace == "" || in.OutputBucket == "" {
		return nil, fmt.Errorf("aivision: video analysis requires output location")
	}

	details := aivision.CreateVideoJobDetails{
		CompartmentId: common.String(in.CompartmentID),
		InputLocation: aivision.ObjectListInlineInputLocation{
			ObjectLocations: []aivision.ObjectLocation{
				{
					NamespaceName: common.String(in.NamespaceName),
					BucketName:    common.String(in.BucketName),
					ObjectName:    common.String(in.ObjectName),
				},
			},
		},
		OutputLocation: &aivision.OutputLocation{
			NamespaceName: common.String(in.OutputNamespace),
			BucketName:    common.String(in.OutputBucket),
			Prefix:        common.String(in.OutputPrefix),
		},
	}
	if in.DisplayName != "" {
		details.DisplayName = common.String(in.DisplayName)
	}

	// Build video features.
	features, err := buildVideoFeatures(in.Features)
	if err != nil {
		return nil, err
	}
	details.Features = features

	resp, err := c.CreateVideoJob(ctx, aivision.CreateVideoJobRequest{
		CreateVideoJobDetails: details,
	})
	if err != nil {
		return nil, fmt.Errorf("aivision: create video job: %w", err)
	}

	return &AnalyzeVideoOutput{
		WorkRequestID: derefStr(resp.Id),
	}, nil
}

// GetVideoAnalysisStatus returns the status of a video analysis job.
func GetVideoAnalysisStatus(ctx context.Context, c *aivision.AIServiceVisionClient, videoJobID string) (*VideoAnalysisStatus, error) {
	resp, err := c.GetVideoJob(ctx, aivision.GetVideoJobRequest{
		VideoJobId: common.String(videoJobID),
	})
	if err != nil {
		return nil, fmt.Errorf("aivision: get video job: %w", err)
	}

	status := &VideoAnalysisStatus{
		WorkRequestID: derefStr(resp.Id),
		Status:        string(resp.LifecycleState),
		DisplayName:   derefStr(resp.DisplayName),
	}
	if resp.PercentComplete != nil {
		status.PercentComplete = *resp.PercentComplete
	}
	if resp.TimeAccepted != nil {
		status.TimeAccepted = resp.TimeAccepted.Time.Format(timeLayout)
	}
	if resp.TimeStarted != nil {
		status.TimeStarted = resp.TimeStarted.Time.Format(timeLayout)
	}
	if resp.TimeFinished != nil {
		status.TimeFinished = resp.TimeFinished.Time.Format(timeLayout)
	}
	return status, nil
}

// CancelVideoAnalysis cancels a video analysis job.
func CancelVideoAnalysis(ctx context.Context, c *aivision.AIServiceVisionClient, videoJobID string) error {
	_, err := c.CancelVideoJob(ctx, aivision.CancelVideoJobRequest{
		VideoJobId: common.String(videoJobID),
	})
	if err != nil {
		return fmt.Errorf("aivision: cancel video job: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Feature builders
// ---------------------------------------------------------------------------

func buildImageFeatures(featureNames []string, maxResults int) ([]aivision.ImageFeature, error) {
	if len(featureNames) == 0 {
		return nil, fmt.Errorf("aivision: at least one feature required")
	}
	var features []aivision.ImageFeature
	for _, name := range featureNames {
		switch name {
		case "CLASSIFICATION":
			f := aivision.ImageClassificationFeature{}
			if maxResults > 0 {
				f.MaxResults = common.Int(maxResults)
			}
			features = append(features, f)
		case "OBJECT_DETECTION":
			f := aivision.ImageObjectDetectionFeature{}
			if maxResults > 0 {
				f.MaxResults = common.Int(maxResults)
			}
			features = append(features, f)
		case "TEXT_RECOGNITION":
			features = append(features, aivision.ImageTextDetectionFeature{})
		default:
			return nil, fmt.Errorf("aivision: unknown image feature: %s", name)
		}
	}
	return features, nil
}

func buildDocumentFeatures(featureNames []string) ([]aivision.DocumentFeature, error) {
	if len(featureNames) == 0 {
		return nil, fmt.Errorf("aivision: at least one document feature required")
	}
	var features []aivision.DocumentFeature
	for _, name := range featureNames {
		switch name {
		case "TABLE_DETECTION":
			features = append(features, aivision.DocumentTableDetectionFeature{})
		case "KEY_VALUE_EXTRACTION":
			features = append(features, aivision.DocumentKeyValueDetectionFeature{})
		case "LANGUAGE_CLASSIFICATION":
			features = append(features, aivision.DocumentLanguageClassificationFeature{})
		default:
			return nil, fmt.Errorf("aivision: unknown document feature: %s", name)
		}
	}
	return features, nil
}

func buildVideoFeatures(featureNames []string) ([]aivision.VideoFeature, error) {
	if len(featureNames) == 0 {
		return nil, fmt.Errorf("aivision: at least one video feature required")
	}
	var features []aivision.VideoFeature
	for _, name := range featureNames {
		switch name {
		case "OBJECT_TRACKING":
			features = append(features, aivision.VideoObjectTrackingFeature{})
		case "LABEL_DETECTION":
			features = append(features, aivision.VideoLabelDetectionFeature{})
		default:
			return nil, fmt.Errorf("aivision: unknown video feature: %s", name)
		}
	}
	return features, nil
}

// ---------------------------------------------------------------------------
// Output builders
// ---------------------------------------------------------------------------

func buildAnalyzeImageOutput(result aivision.AnalyzeImageResult) *AnalyzeImageOutput {
	out := &AnalyzeImageOutput{}

	// Classification labels.
	for _, l := range result.Labels {
		out.ImageClassificationLabels = append(out.ImageClassificationLabels, ClassificationLabel{
			Name:       derefStr(l.Name),
			Confidence: derefFloat32(l.Confidence),
		})
	}

	// Detected objects.
	for _, obj := range result.ImageObjects {
		do := DetectedObject{
			Name:       derefStr(obj.Name),
			Confidence: derefFloat32(obj.Confidence),
		}
		if obj.BoundingPolygon != nil {
			do.BoundingPolygon = convertPolygon(*obj.BoundingPolygon)
		}
		out.DetectedObjects = append(out.DetectedObjects, do)
	}

	// Detected text.
	if result.ImageText != nil {
		for _, line := range result.ImageText.Lines {
			dt := DetectedTextLine{
				Text: derefStr(line.Text),
			}
			if line.Confidence != nil {
				dt.Confidence = *line.Confidence
			}
			if line.BoundingPolygon != nil {
				dt.BoundingPolygon = convertPolygon(*line.BoundingPolygon)
			}
			out.DetectedText = append(out.DetectedText, dt)
		}
	}

	return out
}

func buildAnalyzeDocumentOutput(result aivision.AnalyzeDocumentResult) *AnalyzeDocumentOutput {
	out := &AnalyzeDocumentOutput{}

	for _, page := range result.Pages {
		pvo := DocumentPageVO{
			PageNumber: derefInt(page.PageNumber),
		}
		// Page has Dimensions directly (not DocumentPageMetadata).
		if page.Dimensions != nil {
			pvo.Width = float32(derefFloat64(page.Dimensions.Width))
			pvo.Height = float32(derefFloat64(page.Dimensions.Height))
		}
		// DetectedLanguages is directly on Page.
		for _, lang := range page.DetectedLanguages {
			pvo.Languages = append(pvo.Languages, DocumentLanguageVO{
				Language:   string(lang.LanguageCode),
				Confidence: derefFloat32(lang.Confidence),
			})
		}

		// Tables: Table has HeaderRows/BodyRows/FooterRows, each row has Cells.
		for _, table := range page.Tables {
			tvo := DocumentTableVO{
				RowCount:    derefInt(table.RowCount),
				ColumnCount: derefInt(table.ColumnCount),
			}
			for _, row := range table.BodyRows {
				for _, cell := range row.Cells {
					tvo.Cells = append(tvo.Cells, TableCellVO{
						Text:        derefStr(cell.Text),
						RowIndex:    derefInt(cell.RowIndex),
						ColumnIndex: derefInt(cell.ColumnIndex),
						Confidence:  derefFloat32(cell.Confidence),
					})
				}
			}
			for _, row := range table.HeaderRows {
				for _, cell := range row.Cells {
					tvo.Cells = append(tvo.Cells, TableCellVO{
						Text:        derefStr(cell.Text),
						RowIndex:    derefInt(cell.RowIndex),
						ColumnIndex: derefInt(cell.ColumnIndex),
						Confidence:  derefFloat32(cell.Confidence),
					})
				}
			}
			pvo.Tables = append(pvo.Tables, tvo)
		}

		// Key-value pairs from DocumentFields.
		// DocumentField has FieldType (enum), FieldValue (interface), FieldLabel, FieldName.
		for _, kv := range page.DocumentFields {
			kvp := KeyValuePairVO{}
			// Key: use FieldLabel if available, else FieldName.
			if kv.FieldLabel != nil {
				kvp.Key = FieldVO{
					Text:       derefStr(kv.FieldLabel.Name),
					Confidence: derefFloat32(kv.FieldLabel.Confidence),
				}
			} else if kv.FieldName != nil {
				kvp.Key = FieldVO{
					Text:       derefStr(kv.FieldName.Name),
					Confidence: derefFloat32(kv.FieldName.Confidence),
				}
			}
			// Value: FieldValue is an interface with GetText()/GetConfidence().
			if kv.FieldValue != nil {
				kvp.Value = FieldVO{
					Text:       derefStr(kv.FieldValue.GetText()),
					Confidence: derefFloat32(kv.FieldValue.GetConfidence()),
				}
			}
			kvp.Key.Text = string(kv.FieldType) + ":" + kvp.Key.Text
			pvo.KeyValuePairs = append(pvo.KeyValuePairs, kvp)
		}

		out.Pages = append(out.Pages, pvo)
	}
	return out
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func convertPolygon(p aivision.BoundingPolygon) *PolygonVO {
	pvo := &PolygonVO{}
	for _, v := range p.NormalizedVertices {
		vert := VertexVO{}
		if v.X != nil {
			vert.X = *v.X
		}
		if v.Y != nil {
			vert.Y = *v.Y
		}
		pvo.NormalizedVertices = append(pvo.NormalizedVertices, vert)
	}
	return pvo
}

