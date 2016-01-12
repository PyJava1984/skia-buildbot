package db

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

func TestPerfTrace(t *testing.T) {
	b, err := ioutil.ReadFile(filepath.Join("testdata", "rietveld_response.txt"))
	assert.Nil(t, err)
	m := mockhttpclient.NewURLMock()
	// Mock this only once to confirm that caching works.
	m.MockOnce("https://codereview.chromium.org/api/1490543002", b)

	review := rietveld.New(rietveld.RIETVELD_SKIA_URL, util.NewTimeoutClient())

	vcsCommits := []*vcsinfo.LongCommit{
		&vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    "foofoofoo",
				Author:  "bar@example.com",
				Subject: "some commit",
			},
		},
	}
	vcs := ingestion.MockVCS(vcsCommits)

	builder := &tileBuilder{
		db:        nil,
		vcs:       vcs,
		review:    review,
		reviewURL: "https://codereview.chromium.org",
		cache:     map[string]*rietveld.Issue{},
	}

	now := time.Unix(100, 0)
	commits := []*CommitID{
		&CommitID{
			Timestamp: time.Now().Unix(),
			ID:        "1",
			Source:    "https://codereview.chromium.org/1490543002",
		},
		&CommitID{
			Timestamp: time.Now().Unix(),
			ID:        "foofoofoo",
			Source:    "master",
		},
	}

	long := builder.convertToLongCommits(commits, "master")
	assert.Equal(t, 1, len(long), "Only one commit should match master.")
	assert.Equal(t, "foofoofoo", long[0].ID)
	assert.Equal(t, "some commit", long[0].Desc)
	assert.Equal(t, "bar@example.com", long[0].Author)

	long = builder.convertToLongCommits(commits, "https://codereview.chromium.org/1490543002")
	assert.Equal(t, 1, len(long), "Only one commit should match the trybot.")
	assert.Equal(t, "1", long[0].ID)
	assert.Equal(t, "no merge conflicts here.", long[0].Desc)
	assert.Equal(t, "jcgregorio", long[0].Author)

	long = builder.convertToLongCommits(commits, "")
	assert.Equal(t, 2, len(long), "Both commits should now appear.")
	assert.Equal(t, "1", long[0].ID)
	assert.Equal(t, "no merge conflicts here.", long[0].Desc)
	assert.Equal(t, "jcgregorio", long[0].Author)
	assert.Equal(t, "foofoofoo", long[1].ID)
	assert.Equal(t, "some commit", long[1].Desc)
	assert.Equal(t, "bar@example.com", long[1].Author)

	badCommits := []*CommitID{
		&CommitID{
			Timestamp: now.Add(2 * time.Minute).Unix(),
			ID:        "2",
			Source:    "https://codereview.chromium.org/99999999",
		},
		&CommitID{
			Timestamp: now.Add(3 * time.Minute).Unix(),
			ID:        "barbarbar",
			Source:    "master",
		},
	}
	long = builder.convertToLongCommits(badCommits, "")
	assert.Equal(t, 2, len(long), "Both commits should now appear.")
	assert.Equal(t, "2", long[0].ID)
	assert.Equal(t, "", long[0].Desc)
	assert.Equal(t, "", long[0].Author)
	assert.Equal(t, "barbarbar", long[1].ID)
	assert.Equal(t, "", long[1].Desc)
	assert.Equal(t, "", long[1].Author)
}