import { expect, it } from "vitest";
import { reorderChoices, reorderMutation } from "./reorder";

const items = [
  { id: "pin:a", name: "A", icon: "", kind: "app", status: "missing" },
  {
    id: "pin:g",
    name: "G",
    icon: "",
    kind: "group",
    members: [{ id: "pin:b", name: "B", icon: "", kind: "app" }],
  },
  { id: "pin:c", name: "C", icon: "", kind: "app" },
  { id: "app:90", name: "Running", icon: "", kind: "app" },
];
it("admits structural missing pins but never running or decoration sources", () => {
  expect(reorderMutation(items, "pin:a", "pin:c", "before")).toEqual({
    kind: "moveBefore",
    itemID: "pin:a",
    targetID: "pin:c",
  });
  expect(reorderMutation(items, "app:90", "pin:c", "before")).toBeNull();
  expect(reorderMutation(items, "pin:b", "pin:c", "before")).toBeNull();
  expect(reorderMutation(items, "pin:b", "pin:c", "group")).toEqual({
    kind: "addToGroup",
    itemID: "pin:b",
    targetID: "pin:c",
  });
});
it("offers exact same-owner moves, explicit group choices and removal", () => {
  expect(reorderChoices(items, "pin:b")).toContainEqual({
    label: "Remove from group",
    targetName: "",
    mutation: { kind: "removeFromGroup", itemID: "pin:b", targetID: "pin:g" },
  });
  expect(reorderMutation(items, "pin:g", "pin:b", "group")).toBeNull();
  expect(reorderMutation(items, "pin:b", "pin:g", "group")).toBeNull();
  expect(reorderChoices(items, "pin:a").some((c) => c.mutation.targetID === "app:90")).toBe(false);
});
