package types

type EmbeddingParams struct {
	Model          string
	Input          []string
	Dimensions     *int
	EncodingFormat *EncodingFormat
	Extra          map[string]any
}

type EncodingFormat string

const (
	EncodingFormatFloat  EncodingFormat = "float"
	EncodingFormatBase64 EncodingFormat = "base64"
)

type EmbeddingResponse struct {
	Model      string
	Embeddings []Embedding
	Usage      *Usage
	Extra      map[string]any
}

type Embedding struct {
	Index  int64
	Vector []float64
	Object string
}

type EmbeddingParamsOption func(*EmbeddingParams)

func WithEmbeddingModel(model string) EmbeddingParamsOption {
	return func(e *EmbeddingParams) {
		e.Model = model
	}
}

func WithInput(input []string) EmbeddingParamsOption {
	return func(e *EmbeddingParams) {
		e.Input = input
	}
}

func WithStringInput(input string) EmbeddingParamsOption {
	return func(e *EmbeddingParams) {
		e.Input = append(e.Input, input)
	}
}

func WithDimensions(dimensions int) EmbeddingParamsOption {
	return func(e *EmbeddingParams) {
		e.Dimensions = &dimensions
	}
}

func WithEncodingFormat(format EncodingFormat) EmbeddingParamsOption {
	return func(e *EmbeddingParams) {
		e.EncodingFormat = &format
	}
}

func WithExtra(extra map[string]any) EmbeddingParamsOption {
	return func(e *EmbeddingParams) {
		e.Extra = extra
	}
}

func NewEmbeddingParams(options ...EmbeddingParamsOption) *EmbeddingParams {
	e := &EmbeddingParams{}
	for _, opts := range options {
		opts(e)
	}
	return e
}
