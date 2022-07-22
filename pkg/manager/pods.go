package manager

import "context"

func StartPod(ctx context.Context, epoch int64, key string) NetLocation {
	return NetLocation{
		host: "localhost",
		port: 3333,
	}
}
