//go:build unit

package main

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolNameListParsesCommaSeparatedAndRepeatedValues(t *testing.T) {
	var tools toolNameList
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(&tools, "enable-tools", "test")

	err := fs.Parse([]string{
		"--enable-tools=get_health, search_dashboards",
		"--enable-tools", "list_folders",
		"--enable-tools=,query_datasource,,",
	})
	require.NoError(t, err)

	assert.True(t, tools.IsSet())
	assert.Equal(t, []string{
		"get_health",
		"search_dashboards",
		"list_folders",
		"query_datasource",
	}, tools.Values())
	assert.Equal(t, "get_health,search_dashboards,list_folders,query_datasource", tools.String())
}

func TestToolNameListTracksExplicitEmptyInput(t *testing.T) {
	var tools toolNameList
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(&tools, "enable-tools", "test")

	err := fs.Parse([]string{"--enable-tools=,,"})
	require.NoError(t, err)

	assert.True(t, tools.IsSet())
	assert.Nil(t, tools.Values())
	assert.Equal(t, "", tools.String())
}

func TestToolConfigRegisterOptionsIncludesToolsets(t *testing.T) {
	tc := toolConfig{
		disableWrite:  true,
		optionalTools: true,
		toolsets:      toolNameList{values: []string{"dashboards", "prometheus"}, set: true},
		enableTools:   toolNameList{values: []string{"get_health"}, set: true},
		disableTools:  toolNameList{values: []string{"query_datasource"}, set: true},
	}

	opts := tc.registerOptions()

	assert.False(t, opts.EnableWriteTools)
	assert.True(t, opts.EnableOptionalTools)
	assert.Equal(t, []string{"dashboards", "prometheus"}, opts.Toolsets)
	assert.True(t, opts.ToolsetsSet)
	assert.Equal(t, []string{"get_health"}, opts.EnableTools)
	assert.Equal(t, []string{"query_datasource"}, opts.DisableTools)
}
