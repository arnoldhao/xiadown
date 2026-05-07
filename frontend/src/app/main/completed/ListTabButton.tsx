import * as React from "react";

import {
  DreamSegmentSwitch,
  type DreamSegmentSwitchItem,
} from "@/shared/ui/dream-segment-switch";

type CompletedListSwitchValue = "tasks" | "files";

export function CompletedListViewSwitch(props: {
  value: CompletedListSwitchValue;
  compact: boolean;
  tasksLabel: string;
  filesLabel: string;
  tasksIcon: React.ReactNode;
  filesIcon: React.ReactNode;
  onValueChange: (value: CompletedListSwitchValue) => void;
}) {
  const items: readonly [
    DreamSegmentSwitchItem<CompletedListSwitchValue>,
    DreamSegmentSwitchItem<CompletedListSwitchValue>,
  ] = [
    {
      value: "tasks" as const,
      label: props.tasksLabel,
      icon: props.tasksIcon,
    },
    {
      value: "files" as const,
      label: props.filesLabel,
      icon: props.filesIcon,
    },
  ];

  return (
    <DreamSegmentSwitch
      value={props.value}
      items={items}
      compact={props.compact}
      ariaLabel={`${props.tasksLabel} / ${props.filesLabel}`}
      onValueChange={props.onValueChange}
    />
  );
}
