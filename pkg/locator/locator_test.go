package locator_test

import (
	"testing"

	"github.com/larynjahor/spd/data"
	"github.com/larynjahor/spd/pkg"
	"github.com/larynjahor/spd/pkg/locator"
	"github.com/stretchr/testify/require"
)

func TestLocator_Locate(t *testing.T) {
	locator, err := locator.NewLocator(data.FS, &data.Env)
	require.NoError(t, err)

	t.Run("not found", func(t *testing.T) {
		_, err := locator.GetPath("github.com/foo/spam/pkg")
		require.ErrorIs(t, err, pkg.ErrPackageNotFound)
	})

	t.Run("GOPATH", func(t *testing.T) {
		path, err := locator.GetPath("gitlab.com/company/foo/internal")
		require.NoError(t, err)

		require.Equal(t, "root/gopath/pkg/mod/gitlab.com/company/foo/internal", path)
	})
}
