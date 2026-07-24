"use client";

import { useState, useRef, useEffect, useMemo } from "react";
import { useApi } from "@/lib/api/hooks";
import styles from "./LocationSelector.module.css";

interface Location {
  id: string;
  name: string;
  country_code: string;
}

interface LocationsData {
  locations: Location[];
}

export interface LocationSelectorProps {
  selected: string | null;
  onChange: (id: string) => void;
}

// Location dropdown with search (doc 02 §1.5 Selectors). Fetches /locations;
// shows name + country code. The selected value is persisted in the URL by the
// parent (useGlobalParams). Auto-selects first active location when none is set.
export function LocationSelector({ selected, onChange }: LocationSelectorProps) {
  const { data } = useApi<LocationsData>("/locations");
  const locations = useMemo(() => data?.data?.locations ?? [], [data]);
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const wrapperRef = useRef<HTMLDivElement>(null);

  // Auto-select first location if nothing is selected and locations are loaded.
  useEffect(() => {
    if (!selected && locations.length > 0) {
      onChange(locations[0].id);
    }
  }, [selected, locations, onChange]);

  // Close dropdown on outside click.
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (wrapperRef.current && !wrapperRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const current = locations.find((l) => l.id === selected);
  const filtered = locations.filter(
    (l) => l.name.toLowerCase().includes(search.toLowerCase()),
  );

  return (
    <div className={styles.wrapper} ref={wrapperRef}>
      <button
        type="button"
        className={styles.trigger}
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        {current ? (
          <>
            {current.name}
            <span className={styles.country}>{current.country_code}</span>
          </>
        ) : (
          "Select location"
        )}
      </button>
      {open && (
        <div className={styles.dropdown} role="listbox" aria-label="Locations">
          <input
            className={styles.search}
            type="search"
            placeholder="Search..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            autoFocus
          />
          {filtered.map((loc) => (
            <button
              key={loc.id}
              type="button"
              role="option"
              aria-selected={loc.id === selected}
              className={`${styles.option} ${loc.id === selected ? styles.selected : ""}`}
              onClick={() => {
                onChange(loc.id);
                setOpen(false);
                setSearch("");
              }}
            >
              {loc.name}
              <span className={styles.country}>{loc.country_code}</span>
            </button>
          ))}
          {filtered.length === 0 && (
            <p style={{ padding: "var(--space-sm)", color: "var(--color-text-muted)" }}>
              No locations found.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
