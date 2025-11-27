package notification

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

const (
	scenarioDefinitionFile  = "common/types/notification_scenario.go"
	scenarioRegisterFile    = "notification/scenarioregister/register.go"
	notificationTemplateDir = "notification/tmplmgr/templates"
	channelInternalMessage  = "internal-message"
	channelEmail            = "email"
)

type NotificationScenario struct {
	Name          string
	Channels      []string
	PayloadFields []string
	Templates     map[string]map[string]map[string]string // channel -> lang -> {title, content}
	BuildTags     []string
	Comment       string
}

var notifyGenCmd = &cobra.Command{
	Use:   "notify-gen",
	Short: "Generate message for internal notification",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNotifyGen(cmd, args)
	},
}

func runNotifyGen(_ *cobra.Command, _ []string) error {
	slog.Info("generating notification messages")

	scenarios, err := parseNotificationScenarios()
	if err != nil {
		slog.Error("failed to parse notification scenarios", "error", err)
		return err
	}

	for _, scenario := range scenarios {
		err := generateTemplateFiles(scenario)
		if err != nil {
			slog.Error("failed to generate template files", "error", err)
			return fmt.Errorf("failed to generate template files: %w", err)
		}
	}

	// generate scenario registration
	for _, scenario := range scenarios {
		err := addScenarioRegistration(scenario)
		if err != nil {
			slog.Error("failed to add scenario registration", "error", err)
			return fmt.Errorf("failed to add scenario registration: %w", err)
		}
	}

	return nil
}

// parseCommaSeparatedValues extracts comma-separated values from a line using the given regex pattern
func parseCommaSeparatedValues(line, pattern string) []string {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		valuesStr := strings.TrimSpace(matches[1])
		values := strings.Split(valuesStr, ",")
		for i, value := range values {
			values[i] = strings.TrimSpace(value)
		}
		return values
	}
	return nil
}

func parseNotificationScenarios() ([]NotificationScenario, error) {
	file, err := os.Open(scenarioDefinitionFile)
	if err != nil {
		slog.Error("failed to open scenario file", "error", err)
		return nil, fmt.Errorf("failed to open scenario file: %w", err)
	}
	defer file.Close()

	var scenarios []NotificationScenario
	scanner := bufio.NewScanner(file)

	var currentScenario *NotificationScenario
	var inCommentBlock bool
	var jsonBuffer strings.Builder
	var inJSONBlock bool
	var jsonTag string

	for scanner.Scan() {
		line := scanner.Text()

		// Check if we're starting a new scenario comment block
		if strings.Contains(line, "@Scenario") {
			if currentScenario != nil {
				scenarios = append(scenarios, *currentScenario)
			}
			currentScenario = &NotificationScenario{}
			inCommentBlock = true

			// Extract scenario name
			re := regexp.MustCompile(`@Scenario\s+(\S+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				currentScenario.Name = matches[1]
			}
			continue
		}

		// Check if we're ending the comment block (constant definition)
		if inCommentBlock && strings.Contains(line, "MessageScenario") && strings.Contains(line, "=") {
			if currentScenario != nil {
				scenarios = append(scenarios, *currentScenario)
				currentScenario = nil
			}
			inCommentBlock = false
			continue
		}

		// Parse other notification comments
		if inCommentBlock && currentScenario != nil {
			if strings.Contains(line, "@Channels") {
				currentScenario.Channels = parseCommaSeparatedValues(line, `@Channels\s+(.+)`)
			} else if strings.Contains(line, "@PayloadFields") {
				currentScenario.PayloadFields = parseCommaSeparatedValues(line, `@PayloadFields\s+(.+)`)
			} else if strings.Contains(line, "@Template") {
				// Start collecting JSON for template
				inJSONBlock = true
				jsonTag = "template"
				jsonBuffer.Reset()
				// Extract the opening brace from the current line
				start := strings.Index(line, "{")
				if start != -1 {
					jsonBuffer.WriteString(line[start:])
				}
			} else if strings.Contains(line, "@BuildTags") {
				currentScenario.BuildTags = parseCommaSeparatedValues(line, `@BuildTags\s+(\S+)`)
			} else if inJSONBlock {
				// Continue collecting JSON lines - strip comment prefix and tabs
				cleanLine := strings.TrimSpace(line)
				if strings.HasPrefix(cleanLine, "//") {
					cleanLine = strings.TrimSpace(cleanLine[2:])
				}

				jsonBuffer.WriteString(cleanLine)

				// Check if we've reached the end of JSON block
				if strings.Contains(line, "}") && isJSONComplete(jsonBuffer.String()) {
					jsonStr := cleanJSON(jsonBuffer.String())

					if jsonTag == "template" {
						var templates map[string]map[string]map[string]string
						if err := json.Unmarshal([]byte(jsonStr), &templates); err == nil {
							currentScenario.Templates = templates
						} else {
							slog.Error("failed to parse template JSON", "error", err, "json", jsonStr)
							return nil, fmt.Errorf("failed to parse template JSON: %w", err)
						}
					}

					inJSONBlock = false
					jsonBuffer.Reset()
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("failed to scan file", "error", err)
		return nil, fmt.Errorf("failed to scan file: %w", err)
	}

	return scenarios, nil
}

func cleanJSON(jsonStr string) string {
	// Remove trailing commas before closing braces and brackets
	re := regexp.MustCompile(`,(\s*[}\]])`)
	return re.ReplaceAllString(jsonStr, "$1")
}

func isJSONComplete(jsonStr string) bool {
	// Simple check to see if JSON is complete by counting braces
	openBraces := strings.Count(jsonStr, "{")
	closeBraces := strings.Count(jsonStr, "}")
	return openBraces > 0 && openBraces == closeBraces
}

func generateTemplateFiles(scenario NotificationScenario) error {
	templateDir := filepath.Join(notificationTemplateDir, scenario.Name)

	// Create scenario directory
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		slog.Error("failed to create template directory for scenario", "scenario", scenario.Name, "error", err)
		return fmt.Errorf("failed to create template directory for scenario %s: %w", scenario.Name, err)
	}

	// Generate templates for each channel
	for _, channel := range scenario.Channels {
		// Get templates for this channel
		channelTemplates, exists := scenario.Templates[channel]
		if !exists {
			// Generate default templates if not specified
			slog.Info("no templates specified for channel, skip", "scenario", scenario.Name, "channel", channel)
			continue
		}

		// Generate template files for each language
		for lang, templateData := range channelTemplates {
			templateFile := filepath.Join(templateDir, fmt.Sprintf("%s.%s.tpl", channel, lang))

			// Create template content
			templateContent := createTemplateContent(templateData, channel)

			if err := os.WriteFile(templateFile, []byte(templateContent), 0644); err != nil {
				return fmt.Errorf("failed to generate template file %s: %w", templateFile, err)
			}

			slog.Info("generated template file", "scenario", scenario.Name, "channel", channel, "lang", lang, "templateFile", templateFile)
		}
	}

	return nil
}

func createTemplateContent(templateData map[string]string, channel string) string {
	title := templateData["title"]
	content := templateData["content"]

	// Wrap content in HTML structure if it's an email channel
	var htmlContent string
	if channel == channelEmail {
		htmlContent = fmt.Sprintf(`<html>
	<body>
		<h3>%s</h3>
		<p>%s</p>
	</body>
</html>`, title, content)
	} else {
		htmlContent = content
	}

	// Create the template with title and content sections separated by ---
	templateContent := fmt.Sprintf(`{{/* title section */}}
%s
---
{{/* content section */}}
%s`, title, htmlContent)

	return templateContent
}

func addScenarioRegistration(scenario NotificationScenario) error {
	registration := generateScenarioRegistration(scenario)

	for _, buildTag := range scenario.BuildTags {
		registerFile := getRegisterFile(buildTag)

		// Read the file
		content, err := os.ReadFile(registerFile)
		if err != nil {
			slog.Error("failed to read register file", "registerFile", registerFile, "error", err)
			return fmt.Errorf("failed to read register file: %w", err)
		}

		// Always rewrite scenario registration
		lines := strings.Split(string(content), "\n")

		// Remove any existing registration for this scenario
		var filteredLines []string
		skipUntilEnd := false
		for _, line := range lines {
			// Check if this line starts a registration block for this scenario
			if strings.Contains(line, "// register "+scenario.Name+" scenario") {
				skipUntilEnd = true
				continue
			}

			// If we're in a registration block, skip until we find the end
			if skipUntilEnd {
				if strings.Contains(line, "\t})") {
					skipUntilEnd = false
				}
				continue
			}

			// Keep all other lines
			filteredLines = append(filteredLines, line)
		}
		lines = filteredLines

		// Clean up multiple consecutive empty lines
		var cleanedLines []string
		prevEmpty := false
		for _, line := range lines {
			isEmpty := strings.TrimSpace(line) == ""
			if isEmpty && prevEmpty {
				continue // Skip consecutive empty lines
			}
			cleanedLines = append(cleanedLines, line)
			prevEmpty = isEmpty
		}
		lines = cleanedLines

		// Find insertion point
		insertIndex := -1
		for i := len(lines) - 1; i >= 0; i-- {
			line := lines[i]
			if buildTag == "ce" {
				if strings.Contains(line, "extend(d)") {
					insertIndex = i
					break
				}
			} else {
				if strings.Contains(line, "}") {
					insertIndex = i
					break
				}
			}
		}

		if insertIndex == -1 {
			return fmt.Errorf("could not find insertion point in register file")
		}

		// Insert the new registration
		// Check if we need to add a newline before the registration
		var newLines []string

		// Check if the line before insertion point is not empty
		if insertIndex > 0 && strings.TrimSpace(lines[insertIndex-1]) != "" {
			newLines = append(newLines, "")
		}

		// Add the registration
		newLines = append(newLines, registration)

		// Check if the line after insertion point is empty and remove it to avoid double spacing
		if insertIndex < len(lines) && strings.TrimSpace(lines[insertIndex]) == "" {
			insertIndex++
		}

		lines = append(lines[:insertIndex], append(newLines, lines[insertIndex:]...)...)

		// Write back to file
		newContent := strings.Join(lines, "\n")
		if err := os.WriteFile(registerFile, []byte(newContent), 0644); err != nil {
			slog.Error("failed to write register file", "error", err)
			return fmt.Errorf("failed to write register file: %w", err)
		}
		slog.Info("registered scenario", "scenario", scenario.Name, "build_tag", buildTag)
	}

	return nil
}

func getRegisterFile(buildTag string) string {
	switch buildTag {
	case "ce":
		return scenarioRegisterFile
	default:
		return ""
	}
}

func generateScenarioRegistration(scenario NotificationScenario) string {
	constantName := "types.MessageScenario" + toPascalCase(scenario.Name)

	registration := fmt.Sprintf("\t// register %s scenario\n", scenario.Name)
	registration += fmt.Sprintf("\tscenariomgr.RegisterScenario(%s, &scenariomgr.ScenarioDefinition{\n", constantName)
	registration += "\t\tChannels: []types.MessageChannel{\n"

	for _, channel := range scenario.Channels {
		var channelConstant string
		switch channel {
		case channelInternalMessage:
			channelConstant = "types.MessageChannelInternalMessage"
		case channelEmail:
			channelConstant = "types.MessageChannelEmail"
		}
		registration += fmt.Sprintf("\t\t\t%s,\n", channelConstant)
	}

	registration += "\t\t},\n"
	registration += "\t\tChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{\n"

	for _, channel := range scenario.Channels {
		var channelConstant, dataFunc string
		switch channel {
		case channelInternalMessage:
			channelConstant = "types.MessageChannelInternalMessage"
			dataFunc = "internalnotification.GetSiteInternalMessageData"
		case channelEmail:
			channelConstant = "types.MessageChannelEmail"
			dataFunc = "internalnotification.GetEmailDataFunc(d.GetNotificationStorage())"
		}
		registration += fmt.Sprintf("\t\t\t%s: %s,\n", channelConstant, dataFunc)
	}

	registration += "\t\t},\n"
	registration += "\t})"

	return registration
}

func toPascalCase(s string) string {
	parts := strings.Split(s, "-")
	for i, part := range parts {
		if len(part) > 0 {
			runes := []rune(part)
			runes[0] = unicode.ToUpper(runes[0])
			parts[i] = string(runes)
		}
	}
	return strings.Join(parts, "")
}
