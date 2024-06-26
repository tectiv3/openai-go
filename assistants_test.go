package openai

import (
	"os"
	"testing"
)

// https://platform.openai.com/docs/assistants/overview/step-1-create-an-assistant
const (
	assistantsModel = "gpt-3.5-turbo-1106"
)

func TestAssistants(t *testing.T) {
	_apiKey := os.Getenv("OPENAI_API_KEY")
	_org := os.Getenv("OPENAI_ORGANIZATION")
	_verbose := os.Getenv("VERBOSE")

	client := NewClient(_apiKey, _org)
	client.Verbose = _verbose == "true"

	if len(_apiKey) <= 0 || len(_org) <= 0 {
		t.Errorf("environment variables `OPENAI_API_KEY` and `OPENAI_ORGANIZATION` are needed")
	}

	client.SetBetaHeader(`assistants=v1`)

	// === CreateAssistant ===
	if created, err := client.CreateAssistant(assistantsModel, CreateAssistantOptions{}.
		SetName("My assistant for testing api").
		SetInstructions("You are a helpful assistant.").
		SetTools([]Tool{
			NewFunctionTool(ToolFunction{
				Name:        "get_weather",
				Description: "Determine weather in my location",
				Parameters: NewToolFunctionParameters().
					AddPropertyWithDescription("location", "string", "The city and state e.g. San Francisco, CA").
					AddPropertyWithEnums("unit", "string", "The unit of temperature", []string{"celsius", "fahrenheit"}).
					SetRequiredParameters([]string{"location", "unit"}),
			}),
			NewRetrievalTool(),
		})); err != nil {
		t.Errorf("failed to create assistant: %s", err)
	} else {
		assistantID := created.ID

		// === ListAssistants ===
		if listed, err := client.ListAssistants(nil); err != nil {
			t.Errorf("failed to list assistants: %s", err)
		} else {
			if len(listed.Data) <= 0 {
				t.Errorf("no assistant was fetched while listing")
			}
		}

		// === RetrieveAssistant ===
		if retrieved, err := client.RetrieveAssistant(assistantID); err != nil {
			t.Errorf("failed to fetch assistant: %s", err)
		} else {
			if retrieved.ID != assistantID {
				t.Errorf("retrieved assistant's id: %s differs from the requested one: %s", retrieved.ID, assistantID)
			}

			// === ModifyAssistant ===
			const modifiedDescription = "Determine weather in my location gracefully"
			if modified, err := client.ModifyAssistant(assistantID, ModifyAssistantOptions{}.SetDescription(modifiedDescription)); err != nil {
				t.Errorf("failed to modify assistant: %s", err)
			} else {
				if *modified.Description != modifiedDescription {
					t.Errorf("modified description differs from expectation: %s", *modified.Description)
				}
			}

			if file, err := NewFileParamFromFilepath("./sample/test.rb"); err != nil {
				t.Errorf("failed to open sample file: %s", err)
			} else {
				if uploaded, err := client.UploadFile(file, "assistants"); err != nil {
					t.Errorf("failed to upload file: %s", err)
				} else {
					fileID := uploaded.ID

					// === CreateAssistantFile ===
					if created, err := client.CreateAssistantFile(assistantID, fileID); err != nil {
						t.Errorf("failed to create assistant file: %s", err)
					} else {
						assistantFileID := created.ID

						// === ListAssistantFiles ===
						if listed, err := client.ListAssistantFiles(assistantID, nil); err != nil {
							t.Errorf("failed to list assistant files: %s", err)
						} else {
							if len(listed.Data) <= 0 {
								t.Errorf("no assistant file was fetched while listing")
							}
						}

						// === RetrieveAssistantFile ===
						if retrieved, err := client.RetrieveAssistantFile(assistantID, assistantFileID); err != nil {
							t.Errorf("failed to retrieve assistant file: %s", err)
						} else {
							if retrieved.ID != assistantFileID {
								t.Errorf("retrieved assistant file's id: %s differs from the requested one: %s", retrieved.ID, assistantFileID)
							}

							// === DeleteAssistantFile ===
							if deleted, err := client.DeleteAssistantFile(assistantID, assistantFileID); err != nil {
								t.Errorf("failed to delete assistant file: %s", err)
							} else {
								if !deleted.Deleted {
									t.Errorf("deleted status of deleted assistant file is not true")
								}
							}
						}
					}
				}
			}

			// === DeleteAssistant ===
			if deleted, err := client.DeleteAssistant(assistantID); err != nil {
				t.Errorf("failed to delete assistant: %s", err)
			} else {
				if !deleted.Deleted {
					t.Errorf("deleted status of deleted assistant is not true")
				}
			}
		}
	}
}
