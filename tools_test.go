//go:build unit
// +build unit

package mcpgrafana

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

type testToolParams struct {
	Name     string `json:"name" jsonschema:"required,description=The name parameter"`
	Value    int    `json:"value" jsonschema:"required,description=The value parameter"`
	Optional bool   `json:"optional,omitempty" jsonschema:"description=An optional parameter"`
}

func testToolHandler(ctx context.Context, params testToolParams) (*mcp.CallToolResult, error) {
	if params.Name == "error" {
		return nil, errors.New("test error")
	}
	return mcp.NewToolResultText(params.Name + ": " + string(rune(params.Value))), nil
}

type emptyToolParams struct{}

func emptyToolHandler(ctx context.Context, params emptyToolParams) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("empty"), nil
}

// New handlers for different return types
func stringToolHandler(ctx context.Context, params testToolParams) (string, error) {
	if params.Name == "error" {
		return "", errors.New("test error")
	}
	if params.Name == "empty" {
		return "", nil
	}
	return params.Name + ": " + string(rune(params.Value)), nil
}

func stringPtrToolHandler(ctx context.Context, params testToolParams) (*string, error) {
	if params.Name == "error" {
		return nil, errors.New("test error")
	}
	if params.Name == "nil" {
		return nil, nil
	}
	if params.Name == "empty" {
		empty := ""
		return &empty, nil
	}
	result := params.Name + ": " + string(rune(params.Value))
	return &result, nil
}

type TestResult struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func structToolHandler(ctx context.Context, params testToolParams) (TestResult, error) {
	if params.Name == "error" {
		return TestResult{}, errors.New("test error")
	}
	return TestResult{
		Name:  params.Name,
		Value: params.Value,
	}, nil
}

func structPtrToolHandler(ctx context.Context, params testToolParams) (*TestResult, error) {
	if params.Name == "error" {
		return nil, errors.New("test error")
	}
	if params.Name == "nil" {
		return nil, nil
	}
	return &TestResult{
		Name:  params.Name,
		Value: params.Value,
	}, nil
}

func TestConvertTool(t *testing.T) {
	t.Run("valid handler conversion", func(t *testing.T) {
		tool, handler, err := ConvertTool("test_tool", "A test tool", testToolHandler)

		require.NoError(t, err)
		require.NotNil(t, tool)
		require.NotNil(t, handler)

		// Check tool properties
		assert.Equal(t, "test_tool", tool.Name)
		assert.Equal(t, "A test tool", tool.Description)

		// Check schema properties by marshaling the tool
		toolJSON, err := json.Marshal(tool)
		require.NoError(t, err)

		var toolData map[string]any
		err = json.Unmarshal(toolJSON, &toolData)
		require.NoError(t, err)

		inputSchema, ok := toolData["inputSchema"].(map[string]any)
		require.True(t, ok, "inputSchema should be a map")
		assert.Equal(t, "object", inputSchema["type"])

		properties, ok := inputSchema["properties"].(map[string]any)
		require.True(t, ok, "properties should be a map")
		assert.Contains(t, properties, "name")
		assert.Contains(t, properties, "value")
		assert.Contains(t, properties, "optional")

		// Test handler execution
		ctx := context.Background()
		request := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "test_tool",
				Arguments: map[string]any{
					"name":  "test",
					"value": 65, // ASCII 'A'
				},
			},
		}

		result, err := handler(ctx, request)
		require.NoError(t, err)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test: A", resultString.Text)

		// Test error handling
		errorRequest := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "test_tool",
				Arguments: map[string]any{
					"name":  "error",
					"value": 66,
				},
			},
		}

		result, err = handler(ctx, errorRequest)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		resultString, ok = result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test error", resultString.Text)
	})

	t.Run("empty handler params", func(t *testing.T) {
		tool, handler, err := ConvertTool("empty", "description", emptyToolHandler)

		require.NoError(t, err)
		require.NotNil(t, tool)
		require.NotNil(t, handler)

		// Check tool properties
		assert.Equal(t, "empty", tool.Name)
		assert.Equal(t, "description", tool.Description)

		// Check schema properties by marshaling the tool
		toolJSON, err := json.Marshal(tool)
		require.NoError(t, err)

		var toolData map[string]any
		err = json.Unmarshal(toolJSON, &toolData)
		require.NoError(t, err)

		inputSchema, ok := toolData["inputSchema"].(map[string]any)
		require.True(t, ok, "inputSchema should be a map")
		assert.Equal(t, "object", inputSchema["type"])

		properties, ok := inputSchema["properties"].(map[string]any)
		require.True(t, ok, "properties should be a map")
		assert.Len(t, properties, 0)

		// Test handler execution
		ctx := context.Background()
		request := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "empty",
			},
		}
		result, err := handler(ctx, request)
		require.NoError(t, err)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "empty", resultString.Text)
	})

	t.Run("string return type", func(t *testing.T) {
		_, handler, err := ConvertTool("string_tool", "A string tool", stringToolHandler)
		require.NoError(t, err)

		// Test normal string return
		ctx := context.Background()
		request := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "string_tool",
				Arguments: map[string]any{
					"name":  "test",
					"value": 65, // ASCII 'A'
				},
			},
		}

		result, err := handler(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test: A", resultString.Text)

		// Test empty string return
		emptyRequest := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "string_tool",
				Arguments: map[string]any{
					"name":  "empty",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, emptyRequest)
		require.NoError(t, err)
		assert.Nil(t, result)

		// Test error return
		errorRequest := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "string_tool",
				Arguments: map[string]any{
					"name":  "error",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, errorRequest)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		resultString, ok = result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test error", resultString.Text)
	})

	t.Run("string pointer return type", func(t *testing.T) {
		_, handler, err := ConvertTool("string_ptr_tool", "A string pointer tool", stringPtrToolHandler)
		require.NoError(t, err)

		// Test normal string pointer return
		ctx := context.Background()
		request := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "string_ptr_tool",
				Arguments: map[string]any{
					"name":  "test",
					"value": 65, // ASCII 'A'
				},
			},
		}

		result, err := handler(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test: A", resultString.Text)

		// Test nil string pointer return
		nilRequest := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "string_ptr_tool",
				Arguments: map[string]any{
					"name":  "nil",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, nilRequest)
		require.NoError(t, err)
		assert.Nil(t, result)

		// Test empty string pointer return
		emptyRequest := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "string_ptr_tool",
				Arguments: map[string]any{
					"name":  "empty",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, emptyRequest)
		require.NoError(t, err)
		assert.Nil(t, result)

		// Test error return
		errorRequest := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "string_ptr_tool",
				Arguments: map[string]any{
					"name":  "error",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, errorRequest)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		resultString, ok = result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test error", resultString.Text)
	})

	t.Run("struct return type", func(t *testing.T) {
		_, handler, err := ConvertTool("struct_tool", "A struct tool", structToolHandler)
		require.NoError(t, err)

		// Test normal struct return
		ctx := context.Background()
		request := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "struct_tool",
				Arguments: map[string]any{
					"name":  "test",
					"value": 65, // ASCII 'A'
				},
			},
		}

		result, err := handler(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, resultString.Text, `"name":"test"`)
		assert.Contains(t, resultString.Text, `"value":65`)

		// Test error return
		errorRequest := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "struct_tool",
				Arguments: map[string]any{
					"name":  "error",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, errorRequest)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		resultString, ok = result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test error", resultString.Text)
	})

	t.Run("struct pointer return type", func(t *testing.T) {
		_, handler, err := ConvertTool("struct_ptr_tool", "A struct pointer tool", structPtrToolHandler)
		require.NoError(t, err)

		// Test normal struct pointer return
		ctx := context.Background()
		request := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "struct_ptr_tool",
				Arguments: map[string]any{
					"name":  "test",
					"value": 65, // ASCII 'A'
				},
			},
		}

		result, err := handler(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, resultString.Text, `"name":"test"`)
		assert.Contains(t, resultString.Text, `"value":65`)

		// Test nil struct pointer return
		nilRequest := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "struct_ptr_tool",
				Arguments: map[string]any{
					"name":  "nil",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, nilRequest)
		require.NoError(t, err)
		assert.Nil(t, result)

		// Test error return
		errorRequest := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "struct_ptr_tool",
				Arguments: map[string]any{
					"name":  "error",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, errorRequest)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		resultString, ok = result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test error", resultString.Text)
	})

	t.Run("invalid handler types", func(t *testing.T) {
		// Test wrong second argument type (not a struct)
		wrongSecondArgFunc := func(ctx context.Context, s string) (*mcp.CallToolResult, error) {
			return nil, nil
		}
		_, _, err := ConvertTool("invalid", "description", wrongSecondArgFunc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "second argument must be a struct")
	})

	t.Run("handler execution with invalid arguments", func(t *testing.T) {
		_, handler, err := ConvertTool("test_tool", "A test tool", testToolHandler)
		require.NoError(t, err)

		// Test with invalid JSON
		invalidRequest := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Arguments: map[string]any{
					"name": make(chan int), // Channels can't be marshaled to JSON
				},
			},
		}

		_, err = handler(context.Background(), invalidRequest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "marshal args")

		// Test with type mismatch
		mismatchRequest := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Arguments: map[string]any{
					"name":  123, // Should be a string
					"value": "not an int",
				},
			},
		}

		_, err = handler(context.Background(), mismatchRequest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal args")
	})
}

func TestCreateJSONSchemaFromHandler(t *testing.T) {
	schema := createJSONSchemaFromHandler(testToolHandler)

	assert.Equal(t, "object", schema.Type)
	assert.Len(t, schema.Required, 2) // name and value are required, optional is not

	// Check properties
	nameProperty, ok := schema.Properties.Get("name")
	assert.True(t, ok)
	assert.Equal(t, "string", nameProperty.Type)
	assert.Equal(t, "The name parameter", nameProperty.Description)

	valueProperty, ok := schema.Properties.Get("value")
	assert.True(t, ok)
	assert.Equal(t, "integer", valueProperty.Type)
	assert.Equal(t, "The value parameter", valueProperty.Description)

	optionalProperty, ok := schema.Properties.Get("optional")
	assert.True(t, ok)
	assert.Equal(t, "boolean", optionalProperty.Type)
	assert.Equal(t, "An optional parameter", optionalProperty.Description)
}

func TestEmptyStructJSONSchema(t *testing.T) {
	// Test that empty structs generate correct JSON schema with empty properties object
	tool, _, err := ConvertTool("empty_tool", "An empty tool", emptyToolHandler)
	require.NoError(t, err)

	// Marshal the entire Tool to JSON
	jsonBytes, err := json.Marshal(tool)
	require.NoError(t, err)
	t.Logf("Marshaled Tool JSON: %s", string(jsonBytes))

	// Unmarshal to verify structure
	var unmarshaled map[string]any
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	// Verify that inputSchema exists
	inputSchema, exists := unmarshaled["inputSchema"]
	assert.True(t, exists, "inputSchema field should exist in tool JSON")
	assert.NotNil(t, inputSchema, "inputSchema should not be nil")

	// Verify inputSchema structure
	inputSchemaMap, ok := inputSchema.(map[string]any)
	assert.True(t, ok, "inputSchema should be a map")

	// Verify type is object
	assert.Equal(t, "object", inputSchemaMap["type"], "inputSchema type should be object")

	// Verify that properties key exists and is an empty object
	properties, exists := inputSchemaMap["properties"]
	assert.True(t, exists, "properties field should exist in inputSchema")
	assert.NotNil(t, properties, "properties should not be nil")

	propertiesMap, ok := properties.(map[string]any)
	assert.True(t, ok, "properties should be a map")
	assert.Len(t, propertiesMap, 0, "properties should be an empty map")
}

func TestToolTracingInstrumentation(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(spanRecorder),
	)
	originalProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(tracerProvider)
	defer otel.SetTracerProvider(originalProvider)

	t.Run("successful tool execution creates span with correct attributes", func(t *testing.T) {
		spanRecorder.Reset()

		type TestParams struct {
			Message string `json:"message" jsonschema:"description=Test message"`
		}

		testHandler := func(ctx context.Context, args TestParams) (string, error) {
			return "Hello " + args.Message, nil
		}

		tool := MustTool("test_tool", "A test tool for tracing", testHandler)
		ctx := WithGrafanaConfig(context.Background(), GrafanaConfig{
			IncludeArgumentsInSpans: true,
		})

		request := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "test_tool",
				Arguments: map[string]any{
					"message": "world",
				},
			},
		}

		result, err := tool.Handler(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)

		spans := spanRecorder.Ended()
		require.Len(t, spans, 1)

		span := spans[0]
		assert.Equal(t, "tools/call test_tool", span.Name())
		assert.Equal(t, codes.Ok, span.Status().Code)

		attributes := span.Attributes()
		assertHasAttribute(t, attributes, "gen_ai.tool.name", "test_tool")
		assertHasAttribute(t, attributes, "mcp.method.name", "tools/call")
		assertHasAttribute(t, attributes, "gen_ai.tool.call.arguments", `{"message":"world"}`)
	})

	t.Run("tool execution error records error on span", func(t *testing.T) {
		spanRecorder.Reset()

		type TestParams struct {
			ShouldFail bool `json:"shouldFail" jsonschema:"description=Whether to fail"`
		}

		testHandler := func(ctx context.Context, args TestParams) (string, error) {
			if args.ShouldFail {
				return "", assert.AnError
			}
			return "success", nil
		}

		tool := MustTool("failing_tool", "A tool that can fail", testHandler)
		ctx := WithGrafanaConfig(context.Background(), GrafanaConfig{})

		request := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "failing_tool",
				Arguments: map[string]any{
					"shouldFail": true,
				},
			},
		}

		result, err := tool.Handler(ctx, request)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)

		spans := spanRecorder.Ended()
		require.Len(t, spans, 1)

		span := spans[0]
		assert.Equal(t, "tools/call failing_tool", span.Name())
		assert.Equal(t, codes.Error, span.Status().Code)
		assert.Equal(t, assert.AnError.Error(), span.Status().Description)

		hasErrorEvent := false
		for _, event := range span.Events() {
			if event.Name == "exception" {
				hasErrorEvent = true
				break
			}
		}
		assert.True(t, hasErrorEvent, "expected error event to be recorded on span")
	})

	t.Run("spans always created for context propagation", func(t *testing.T) {
		spanRecorder.Reset()

		type TestParams struct {
			Message string `json:"message" jsonschema:"description=Test message"`
		}

		testHandler := func(ctx context.Context, args TestParams) (string, error) {
			return "processed", nil
		}

		tool := MustTool("context_prop_tool", "A tool for context propagation", testHandler)
		ctx := WithGrafanaConfig(context.Background(), GrafanaConfig{})

		request := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "context_prop_tool",
				Arguments: map[string]any{
					"message": "test",
				},
			},
		}

		result, err := tool.Handler(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)

		spans := spanRecorder.Ended()
		require.Len(t, spans, 1)

		span := spans[0]
		assert.Equal(t, "tools/call context_prop_tool", span.Name())
		assert.Equal(t, codes.Ok, span.Status().Code)
	})

	t.Run("arguments not logged by default", func(t *testing.T) {
		spanRecorder.Reset()

		type TestParams struct {
			SensitiveData string `json:"sensitiveData" jsonschema:"description=Potentially sensitive data"`
		}

		testHandler := func(ctx context.Context, args TestParams) (string, error) {
			return "processed", nil
		}

		tool := MustTool("sensitive_tool", "A tool with sensitive data", testHandler)
		ctx := WithGrafanaConfig(context.Background(), GrafanaConfig{
			IncludeArgumentsInSpans: false,
		})

		request := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "sensitive_tool",
				Arguments: map[string]any{
					"sensitiveData": "user@example.com",
				},
			},
		}

		result, err := tool.Handler(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)

		spans := spanRecorder.Ended()
		require.Len(t, spans, 1)

		span := spans[0]
		assert.Equal(t, "tools/call sensitive_tool", span.Name())
		assert.Equal(t, codes.Ok, span.Status().Code)

		attributes := span.Attributes()
		assertHasAttribute(t, attributes, "gen_ai.tool.name", "sensitive_tool")
		assertHasAttribute(t, attributes, "mcp.method.name", "tools/call")
		for _, attr := range attributes {
			assert.NotEqual(t, "gen_ai.tool.call.arguments", string(attr.Key), "arguments should not be logged by default")
		}
	})

	t.Run("arguments logged when argument logging enabled", func(t *testing.T) {
		spanRecorder.Reset()

		type TestParams struct {
			SafeData string `json:"safeData" jsonschema:"description=Non-sensitive data"`
		}

		testHandler := func(ctx context.Context, args TestParams) (string, error) {
			return "processed", nil
		}

		tool := MustTool("debug_tool", "A tool for debugging", testHandler)
		ctx := WithGrafanaConfig(context.Background(), GrafanaConfig{
			IncludeArgumentsInSpans: true,
		})

		request := mcp.CallToolRequest{
			Params: struct {
				Name      string    `json:"name"`
				Arguments any       `json:"arguments,omitempty"`
				Meta      *mcp.Meta `json:"_meta,omitempty"`
			}{
				Name: "debug_tool",
				Arguments: map[string]any{
					"safeData": "debug-value",
				},
			},
		}

		result, err := tool.Handler(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)

		spans := spanRecorder.Ended()
		require.Len(t, spans, 1)

		span := spans[0]
		assert.Equal(t, "tools/call debug_tool", span.Name())
		assert.Equal(t, codes.Ok, span.Status().Code)

		attributes := span.Attributes()
		assertHasAttribute(t, attributes, "gen_ai.tool.name", "debug_tool")
		assertHasAttribute(t, attributes, "mcp.method.name", "tools/call")
		assertHasAttribute(t, attributes, "gen_ai.tool.call.arguments", `{"safeData":"debug-value"}`)
	})
}

func TestExtractTraceContext(t *testing.T) {
	t.Run("no meta returns original context", func(t *testing.T) {
		ctx := context.Background()
		request := mcp.CallToolRequest{}
		result := extractTraceContext(ctx, request)
		assert.Equal(t, ctx, result)
	})

	t.Run("empty meta returns original context", func(t *testing.T) {
		ctx := context.Background()
		request := mcp.CallToolRequest{}
		request.Params.Meta = &mcp.Meta{}
		result := extractTraceContext(ctx, request)
		assert.Equal(t, ctx, result)
	})

	t.Run("valid traceparent extracts span context", func(t *testing.T) {
		ctx := context.Background()
		request := mcp.CallToolRequest{}
		request.Params.Meta = &mcp.Meta{
			AdditionalFields: map[string]any{
				"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			},
		}
		result := extractTraceContext(ctx, request)

		sc := trace.SpanContextFromContext(result)
		assert.True(t, sc.IsValid())
		assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", sc.TraceID().String())
		assert.Equal(t, "00f067aa0ba902b7", sc.SpanID().String())
	})

	t.Run("invalid traceparent returns context unchanged", func(t *testing.T) {
		ctx := context.Background()
		request := mcp.CallToolRequest{}
		request.Params.Meta = &mcp.Meta{
			AdditionalFields: map[string]any{
				"traceparent": "not-a-valid-traceparent",
			},
		}
		result := extractTraceContext(ctx, request)
		sc := trace.SpanContextFromContext(result)
		assert.False(t, sc.IsValid())
	})
}

func assertHasAttribute(t *testing.T, attributes []attribute.KeyValue, key string, expectedValue string) {
	t.Helper()
	for _, attr := range attributes {
		if string(attr.Key) == key {
			assert.Equal(t, expectedValue, attr.Value.AsString())
			return
		}
	}
	t.Errorf("expected attribute %s with value %s not found", key, expectedValue)
}
