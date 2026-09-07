import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { makeT } from "../lib/i18n";
import { emptyState, type MediaViewState } from "../lib/types";
import { MediaPanel } from "./MediaPanel";

const state: MediaViewState = {
  session: 9,
  revision: 4,
  open: true,
  pinned: false,
  pinnable: true,
  provider: "music",
  scope: {
    provider: "music",
    process: { pid: 3, launchID: "x" },
    generation: 2,
    trackEpoch: 7,
    trackID: "A",
  },
  sample: {
    provider: "music",
    process: { pid: 3, launchID: "x" },
    generation: 2,
    sequence: 8,
    trackEpoch: 7,
    track: { id: "A", title: "Night Lines", artist: "Mira", album: "North", durationMS: 10000 },
    playback: "paused",
    positionMS: 2500,
    observedAt: "",
    status: "ready",
    reason: "",
    capabilities: { play: true, pause: true, previous: true, next: true, seek: true },
    artworkToken: "opaque",
  },
  appearance: emptyState.appearance,
  artwork: { status: "networkDisabled", reason: "Remote artwork is off", image: "" },
  lyrics: {
    documentID: "doc",
    status: "ready",
    reason: "",
    cues: [
      { atMs: 0, text: "First" },
      { atMs: 2000, text: "Current" },
      { atMs: 4000, text: "Next" },
    ],
    offsetMS: 0,
  },
  positionMS: 2500,
  activeCue: 1,
  error: "",
};

const handlers = () => ({
  onAction: vi.fn(),
  onPin: vi.fn(),
  onClose: vi.fn(),
  onSize: vi.fn(),
  onImport: vi.fn(),
  onCancelImport: vi.fn(),
  onReload: vi.fn(),
  onRemove: vi.fn(),
  onOffset: vi.fn(),
});

describe("MediaPanel", () => {
  it("localizes icon-only hover and pinned media controls", () => {
    const h = handlers();
    const first = render(<MediaPanel state={state} handlers={h} t={makeT("pt-BR")} />);
    expect(screen.getByRole("button", { name: "Reproduzir" })).toBeInTheDocument();
    expect(screen.getByRole("slider", { name: "Posição da reprodução" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Fixar painel de mídia" })).toBeInTheDocument();
    first.unmount();
    render(
      <MediaPanel
        state={{ ...state, pinned: true, pinnable: false }}
        handlers={h}
        t={makeT("es")}
      />,
    );
    expect(screen.getByRole("button", { name: "Cerrar panel multimedia" })).toBeInTheDocument();
  });

  it("sends explicit transport and seek requests and renders bounded lyric context", () => {
    const h = handlers();
    render(<MediaPanel state={state} handlers={h} />);
    fireEvent.click(screen.getByRole("button", { name: "Play" }));
    expect(h.onAction).toHaveBeenCalledWith(9, 4, "play", 0);
    const seek = screen.getByRole("slider", { name: "Playback position" });
    fireEvent.change(seek, { target: { value: "9999" } });
    fireEvent.pointerUp(seek);
    expect(h.onAction).toHaveBeenCalledWith(9, 4, "seek", 9999);
    expect(screen.getByText("Current")).toHaveAttribute("aria-current", "true");
  });

  it("drops a seek release after the track scope changes and never exposes remote URLs", () => {
    const h = handlers();
    const { rerender } = render(<MediaPanel state={state} handlers={h} />);
    const seek = screen.getByRole("slider", { name: "Playback position" });
    fireEvent.change(seek, { target: { value: "7000" } });
    rerender(
      <MediaPanel
        state={{
          ...state,
          scope: { ...state.scope, trackID: "B", trackEpoch: 8 },
          sample: { ...state.sample, trackEpoch: 8, track: { ...state.sample.track, id: "B" } },
        }}
        handlers={h}
      />,
    );
    fireEvent.pointerUp(seek);
    expect(h.onAction).not.toHaveBeenCalled();
    expect(document.querySelector("img[src^='http']")).toBeNull();
    expect(screen.getByText("Remote artwork is off")).toBeInTheDocument();
  });

  it("bounds long lyric rendering and restores follow after manual scrolling", () => {
    const cues = Array.from({ length: 10_000 }, (_, i) => ({ atMs: i * 1000, text: `Line ${i}` }));
    render(
      <MediaPanel
        state={{ ...state, activeCue: 5000, lyrics: { ...state.lyrics, cues } }}
        handlers={handlers()}
      />,
    );
    expect(document.querySelectorAll(".ot-media-lyrics li")).toHaveLength(25);
    fireEvent.wheel(document.querySelector(".ot-media-lyrics")!);
    fireEvent.scroll(document.querySelector(".ot-media-lyrics")!);
    expect(screen.getByRole("button", { name: "Follow" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Follow" }));
    expect(screen.queryByRole("button", { name: "Follow" })).toBeNull();
  });

  it("cancels an accepted import with its captured revision", async () => {
    let finish!: () => void;
    const pending = new Promise<void>((resolve) => {
      finish = resolve;
    });
    const h = handlers();
    h.onImport.mockReturnValue(pending);
    const { rerender } = render(
      <MediaPanel
        state={{
          ...state,
          interactionEpoch: 1,
          lyrics: { ...state.lyrics, documentID: "", status: "missing", cues: [] },
        }}
        handlers={h}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Import .lrc" }));
    rerender(
      <MediaPanel
        state={{
          ...state,
          revision: 5,
          interactionEpoch: 2,
          lyrics: { ...state.lyrics, documentID: "", status: "missing", cues: [] },
        }}
        handlers={h}
      />,
    );
    fireEvent.click(await screen.findByRole("button", { name: "Cancel import" }));
    expect(h.onCancelImport).toHaveBeenCalledWith(9, 4);
    await act(async () => finish());
  });

  it("freezes the lyric reading window after a user scroll until Follow", () => {
    const cues = Array.from({ length: 100 }, (_, i) => ({ atMs: i, text: `Line ${i}` }));
    const { rerender } = render(
      <MediaPanel
        state={{ ...state, activeCue: 30, lyrics: { ...state.lyrics, cues } }}
        handlers={handlers()}
      />,
    );
    const list = document.querySelector(".ot-media-lyrics")!;
    fireEvent.wheel(list);
    fireEvent.scroll(list);
    rerender(
      <MediaPanel
        state={{ ...state, activeCue: 70, lyrics: { ...state.lyrics, cues } }}
        handlers={handlers()}
      />,
    );
    expect(screen.getByText("Line 30")).toBeInTheDocument();
    expect(screen.queryByText("Line 70")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Follow" }));
    expect(screen.getByText("Line 70")).toBeInTheDocument();
  });

  it("tracks Replace as one session-scoped import and ignores another start", async () => {
    let finish!: () => void;
    const h = handlers();
    h.onImport.mockReturnValue(
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
    );
    const { rerender } = render(<MediaPanel state={state} handlers={h} />);
    fireEvent.click(screen.getByRole("button", { name: "Replace" }));
    fireEvent.click(screen.getByRole("button", { name: "Replace" }));
    expect(h.onImport).toHaveBeenCalledTimes(1);
    rerender(<MediaPanel state={{ ...state, session: 10 }} handlers={h} />);
    expect(screen.queryByRole("button", { name: "Cancel import" })).toBeNull();
    await act(async () => finish());
  });

  it("commits keyboard seek once and retires drafts on cancellation or interaction epoch", () => {
    const h = handlers();
    const { rerender } = render(
      <MediaPanel state={{ ...state, interactionEpoch: 1 }} handlers={h} />,
    );
    let seek = screen.getByRole("slider", { name: "Playback position" });
    fireEvent.change(seek, { target: { value: "6000" } });
    fireEvent.keyUp(seek, { key: "Home" });
    fireEvent.keyUp(seek, { key: "ArrowRight" });
    expect(h.onAction).toHaveBeenCalledTimes(1);
    fireEvent.change(seek, { target: { value: "7000" } });
    fireEvent.pointerCancel(seek);
    fireEvent.pointerUp(seek);
    expect(h.onAction).toHaveBeenCalledTimes(1);
    fireEvent.change(seek, { target: { value: "8000" } });
    rerender(<MediaPanel state={{ ...state, interactionEpoch: 2 }} handlers={h} />);
    seek = screen.getByRole("slider", { name: "Playback position" });
    fireEvent.pointerUp(seek);
    expect(h.onAction).toHaveBeenCalledTimes(1);
    fireEvent.change(seek, { target: { value: "9000" } });
    rerender(<MediaPanel state={{ ...state, session: 10, interactionEpoch: 2 }} handlers={h} />);
    fireEvent.pointerUp(screen.getByRole("slider", { name: "Playback position" }));
    expect(h.onAction).toHaveBeenCalledTimes(1);
  });

  it("reserves the native pinned header exclusion and measures the complete host", () => {
    const h = handlers();
    const rect = vi
      .spyOn(HTMLElement.prototype, "getBoundingClientRect")
      .mockImplementation(function (this: HTMLElement) {
        return DOMRect.fromRect(
          this.classList.contains("ot-dock-panel")
            ? { width: 500, height: 300 }
            : { width: 420, height: 270 },
        );
      });
    render(
      <div className="ot-dock-panel">
        <MediaPanel state={{ ...state, pinned: true, pinnable: false }} handlers={h} />
      </div>,
    );
    expect(h.onSize).toHaveBeenCalledWith(9, 4, 500, 300);
    const header = document.querySelector<HTMLElement>(".ot-media-header")!;
    const close = screen.getByRole("button", { name: "Close media panel" });
    expect(getComputedStyle(header).height).toBe("32px");
    expect(getComputedStyle(close).width).toBe("48px");
    expect(getComputedStyle(close).right).toBe("0px");
    rect.mockRestore();
  });
});
