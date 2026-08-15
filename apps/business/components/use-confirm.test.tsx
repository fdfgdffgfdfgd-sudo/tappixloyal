import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, test, vi } from "vitest";
import { useConfirm } from "./use-confirm";

function Harness({ onResult }: { onResult: (v: boolean) => void }) {
  const { ask, dialog } = useConfirm();
  return (
    <>
      <button
        type="button"
        onClick={async () =>
          onResult(
            await ask({
              title: "Удалить файл?",
              description: "Его нельзя будет восстановить.",
              confirmLabel: "Удалить",
            }),
          )
        }
      >
        Удалить
      </button>
      {dialog}
    </>
  );
}

async function open(onResult = vi.fn()) {
  render(<Harness onResult={onResult} />);
  await userEvent.click(screen.getByRole("button", { name: "Удалить" }));
  return onResult;
}

describe("useConfirm", () => {
  test("asks before going ahead, and reports the answer", async () => {
    const onResult = await open();
    const dialog = within(await screen.findByRole("alertdialog"));
    expect(screen.getByText("Его нельзя будет восстановить.")).toBeInTheDocument();

    // The page trigger is also called "Удалить"; press the one in the dialog.
    await userEvent.click(dialog.getByRole("button", { name: "Удалить" }));
    expect(onResult).toHaveBeenCalledWith(true);
  });

  test("reports a refusal when cancelled", async () => {
    const onResult = await open();
    await userEvent.click(await screen.findByRole("button", { name: "Отмена" }));
    expect(onResult).toHaveBeenCalledWith(false);
  });

  test("Escape counts as a refusal, never as consent", async () => {
    const onResult = await open();
    await screen.findByRole("alertdialog");
    await userEvent.keyboard("{Escape}");
    expect(onResult).toHaveBeenCalledWith(false);
  });

  test("closes after answering", async () => {
    await open();
    await userEvent.click(await screen.findByRole("button", { name: "Отмена" }));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  test("is announced as a dialog that interrupts", async () => {
    await open();
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(dialog).toHaveAccessibleName("Удалить файл?");
    expect(dialog).toHaveAccessibleDescription("Его нельзя будет восстановить.");
  });
});
