// The steps of itos go and itos guide (guide.feature, slice 64): a file
// committed with the text it holds, as a repository keeps its own
// orchestrating notes, and the config key that names another such file.
package features

import "github.com/cucumber/godog"

func initializeGuideSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the committed file "([^"]*)" holding "([^"]*)"$`, w.committedFileHolding)
	sc.Step(`^guide\.orchestrating is "([^"]*)"$`, func(path string) error {
		return w.configSets("guide.orchestrating", path)
	})
}

// The file at the path, from the repository's top, holding the text as one
// line, committed with everything else the scenario has written so far.
func (w *world) committedFileHolding(path, text string) error {
	if err := w.write(path, text+"\n"); err != nil {
		return err
	}
	return w.commit("docs: add " + path)
}
