package model

import "context"

type Localizer interface {
	LocalizedName(context.Context) (string, error)
}
