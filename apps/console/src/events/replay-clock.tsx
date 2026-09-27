import {
  createContext,
  useContext,
  useMemo,
  useState,
  type Dispatch,
  type SetStateAction,
  type ReactNode,
} from "react";
import { timestampDate, type Timestamp } from "@bufbuild/protobuf/wkt";

type Position = { eventId: string; index: number; at: Timestamp } | null;
const Clock = createContext<{
  position: Position;
  setPosition: Dispatch<SetStateAction<Position>>;
} | null>(null);

export function ReplayClockProvider({ children }: { children: ReactNode }) {
  const [position, setPosition] = useState<Position>(null);
  const value = useMemo(() => ({ position, setPosition }), [position]);
  return <Clock value={value}>{children}</Clock>;
}

export function useReplayClock() {
  const clock = useContext(Clock);
  if (!clock) throw new Error("Replay clock provider required");
  return clock;
}

export function ReplayClock() {
  const { position, setPosition } = useReplayClock();
  if (!position) return null;
  return (
    <section className="error-notice" aria-label="Historical geography clock">
      <p>
        Historical geography · {timestampDate(position.at).toISOString()} ·
        Event {position.eventId}. Current operational panels keep their own
        timestamps.
      </p>
      <button className="secondary-button" onClick={() => setPosition(null)}>
        Return to current geography
      </button>
    </section>
  );
}
