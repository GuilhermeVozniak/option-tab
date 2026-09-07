package media

import (
	"math"
	"testing"
	"time"

	"option-tab/internal/platform"
)

type fakeTimelineClock struct{ at time.Time }

func (c *fakeTimelineClock) Now() time.Time          { return c.at }
func (c *fakeTimelineClock) Advance(d time.Duration) { c.at = c.at.Add(d) }

func mediaSample(provider platform.MediaProvider, id, playback string, epoch uint64, position, duration int64, at time.Time) platform.MediaSample {
	return platform.MediaSample{Provider: provider, TrackEpoch: epoch, Track: platform.MediaTrack{ID: id, DurationMS: duration}, Playback: playback, PositionMS: position, ObservedAt: at}
}

func TestTimelineExtrapolatesFreezesAndCorrectsDrift(t *testing.T) {
	clock := &fakeTimelineClock{at: time.Unix(10, 0)}
	timeline := NewTimeline(clock.Now)
	if err := timeline.Load(LyricsScope{Provider: "music", TrackID: "A"}, 1, []Cue{{AtMS: 0, Text: "zero"}, {AtMS: 2500, Text: "later"}}, 0); err != nil {
		t.Fatal(err)
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "A", "playing", 1, 1000, 5000, clock.Now()))
	clock.Advance(1500 * time.Millisecond)
	if got := timeline.PositionMS(); got != 2500 || timeline.Active() != 1 {
		t.Fatalf("playing position=%d cue=%d", got, timeline.Active())
	}
	clock.Advance(5 * time.Second)
	if got := timeline.PositionMS(); got != 3000 {
		t.Fatalf("stale extrapolation did not freeze at 2s: %d", got)
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "A", "playing", 1, 1800, 5000, clock.Now()))
	if got := timeline.PositionMS(); got != 1800 {
		t.Fatalf("fresh sample did not correct drift: %d", got)
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "A", "paused", 1, 3200, 5000, clock.Now()))
	clock.Advance(time.Second)
	if got := timeline.PositionMS(); got != 3200 {
		t.Fatalf("paused position moved: %d", got)
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "A", "playing", 1, 3300, 5000, clock.Now()))
	clock.Advance(400 * time.Millisecond)
	if got := timeline.PositionMS(); got != 3700 {
		t.Fatalf("resumed position=%d", got)
	}
}

func TestTimelineSeeksClampsAndAppliesAssociationOffset(t *testing.T) {
	clock := &fakeTimelineClock{at: time.Unix(20, 0)}
	timeline := NewTimeline(clock.Now)
	if err := timeline.Load(LyricsScope{Provider: "spotify", TrackID: "A"}, 4, []Cue{{AtMS: 1000, Text: "one"}, {AtMS: 3000, Text: "three"}}, 500); err != nil {
		t.Fatal(err)
	}
	timeline.Observe(mediaSample(platform.MediaSpotify, "A", "playing", 4, 3600, 4000, clock.Now()))
	if timeline.Active() != 1 {
		t.Fatalf("offset cue=%d", timeline.Active())
	}
	timeline.Observe(mediaSample(platform.MediaSpotify, "A", "playing", 4, -200, 4000, clock.Now()))
	if timeline.PositionMS() != 0 {
		t.Fatal("negative seek was not clamped")
	}
	timeline.Observe(mediaSample(platform.MediaSpotify, "A", "playing", 4, 9000, 4000, clock.Now()))
	if timeline.PositionMS() != 4000 {
		t.Fatal("forward seek was not clamped to duration")
	}
	if err := timeline.Load(LyricsScope{Provider: "spotify", TrackID: "A"}, 4, nil, 30001); err == nil {
		t.Fatal("accepted association offset beyond 30s")
	}
}

func TestTimelinePositiveOffsetDelaysFirstCue(t *testing.T) {
	clock := &fakeTimelineClock{at: time.Unix(25, 0)}
	timeline := NewTimeline(clock.Now)
	if err := timeline.Load(LyricsScope{Provider: "music", TrackID: "A"}, 1, []Cue{{AtMS: 0, Text: "first"}}, 500); err != nil {
		t.Fatal(err)
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "A", "paused", 1, 0, 1000, clock.Now()))
	if got := timeline.Active(); got != -1 {
		t.Fatalf("delayed first cue activated early: %d", got)
	}
}

func TestTimelinePositionAndActiveUseOneClockReading(t *testing.T) {
	base := time.Unix(26, 0)
	calls := 0
	timeline := NewTimeline(func() time.Time {
		calls++
		return base.Add(time.Duration(calls) * time.Millisecond)
	})
	if err := timeline.Load(LyricsScope{Provider: "music", TrackID: "A"}, 1, []Cue{{AtMS: 0, Text: "before"}, {AtMS: 1001, Text: "boundary"}}, 0); err != nil {
		t.Fatal(err)
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "A", "playing", 1, 1000, 5000, base))
	position, active := timeline.PositionAndActive()
	if calls != 1 || position != 1001 || active != 1 {
		t.Fatalf("calls=%d position=%d active=%d", calls, position, active)
	}
}

func TestTimelineClearsCuesOnEveryTrackEpochIncludingABA(t *testing.T) {
	clock := &fakeTimelineClock{at: time.Unix(30, 0)}
	timeline := NewTimeline(clock.Now)
	if err := timeline.Load(LyricsScope{Provider: "music", TrackID: "A"}, 1, []Cue{{AtMS: 0, Text: "A"}}, 0); err != nil {
		t.Fatal(err)
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "A", "playing", 1, 0, 1000, clock.Now()))
	if timeline.Active() != 0 {
		t.Fatal("A lyrics unavailable")
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "B", "playing", 2, 0, 1000, clock.Now()))
	if timeline.Active() != -1 || timeline.Cues() != nil {
		t.Fatal("A cues survived B")
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "A", "playing", 3, 0, 1000, clock.Now()))
	if timeline.Active() != -1 {
		t.Fatal("old A cues revived after A-B-A")
	}
	if err := timeline.Load(LyricsScope{Provider: "music", TrackID: "A"}, 3, []Cue{{AtMS: 0, Text: "new A"}}, -30000); err != nil {
		t.Fatal(err)
	}
	copy := timeline.Cues()
	copy[0].Text = "mutated"
	if timeline.Cues()[0].Text != "new A" {
		t.Fatal("timeline exposed mutable cues")
	}
	timeline.Clear()
	if timeline.Active() != -1 || timeline.PositionMS() != 0 {
		t.Fatal("clear retained track state")
	}
}

func TestTimelineRejectsOldLyricsAndHandlesUnknownDurationConservatively(t *testing.T) {
	clock := &fakeTimelineClock{at: time.Unix(40, 0)}
	timeline := NewTimeline(clock.Now)
	timeline.Observe(mediaSample(platform.MediaMusic, "B", "playing", 2, 9000, 0, clock.Now()))
	clock.Advance(time.Second)
	if got := timeline.PositionMS(); got != 10000 {
		t.Fatalf("unknown duration was clamped: %d", got)
	}
	if err := timeline.Load(LyricsScope{Provider: "music", TrackID: "A"}, 1, []Cue{{AtMS: 0, Text: "old"}}, 0); err == nil {
		t.Fatal("old track lyrics adopted the current sample")
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "B", "playing", 2, 1200, 0, clock.Now().Add(time.Second)))
	if got := timeline.PositionMS(); got != 1200 {
		t.Fatalf("future sample extrapolated: %d", got)
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "B", "stopped", 2, 700, 0, clock.Now()))
	clock.Advance(time.Second)
	if got := timeline.PositionMS(); got != 700 {
		t.Fatalf("stopped sample moved: %d", got)
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "B", "playing", 2, 800, 0, time.Time{}))
	clock.Advance(time.Second)
	if got := timeline.PositionMS(); got != 800 {
		t.Fatalf("sample without observation time extrapolated: %d", got)
	}
	timeline.Observe(mediaSample(platform.MediaMusic, "B", "playing", 2, math.MaxInt64-10, 0, clock.Now()))
	clock.Advance(time.Second)
	if got := timeline.PositionMS(); got != math.MaxInt64 {
		t.Fatalf("overflowing extrapolation was not saturated: %d", got)
	}
}
