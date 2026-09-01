package v0

import (
	"encoding/base64"
	"strings"

	torrentsTypes "github.com/nyakaspeter/white-raven/server/pkg/torrents/types"
)

func getSourceParams(providers string) torrentsTypes.SourceParams {
	sourceParams := torrentsTypes.SourceParams{}

	for _, source := range strings.Split(providers, ",") {
		sourceName, sourceArgs := getSourceArgs(source)

		switch sourceName {
		case "jackett":
			sourceParams.Jackett.Enabled = true
			if len(sourceArgs) == 2 {
				sourceParams.Jackett.Address = sourceArgs[0]
				sourceParams.Jackett.ApiKey = sourceArgs[1]
			}
		case "ncore":
			sourceParams.Ncore.Enabled = true
			if len(sourceArgs) == 2 {
				sourceParams.Ncore.Username = sourceArgs[0]
				sourceParams.Ncore.Password = sourceArgs[1]
			}
		case "insane":
			sourceParams.Insane.Enabled = true
			if len(sourceArgs) == 2 {
				sourceParams.Insane.Username = sourceArgs[0]
				sourceParams.Insane.Password = sourceArgs[1]
			}
		case "torrentio":
			sourceParams.Torrentio.Enabled = true
		}
	}

	return sourceParams
}

func getSourceArgs(source string) (string, []string) {
	split := strings.Split(source, ":")
	sourceName := strings.ToLower(split[0])
	var decodedArgs []string

	for i := 1; i < len(split); i++ {
		if split[i] == "" {
			continue
		}

		decodedArg, err := base64.StdEncoding.DecodeString(split[i])
		if err == nil {
			decodedArgs = append(decodedArgs, string(decodedArg))
		}
	}

	return sourceName, decodedArgs
}
