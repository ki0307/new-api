/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import {
  PromptInput,
  PromptInputActionAddAttachments,
  PromptInputActionMenu,
  PromptInputActionMenuContent,
  PromptInputActionMenuTrigger,
} from '../prompt-input'

afterEach(cleanup)

test.each(['mouse', 'keyboard'])(
  '%s activation of add attachments opens the file picker',
  async (input) => {
    const user = userEvent.setup()
    render(
      <PromptInput onSubmit={() => undefined}>
        <PromptInputActionMenu>
          <PromptInputActionMenuTrigger aria-label='Attachments' />
          <PromptInputActionMenuContent>
            <PromptInputActionAddAttachments label='Add file' />
          </PromptInputActionMenuContent>
        </PromptInputActionMenu>
      </PromptInput>
    )
    const fileInput = screen.getByLabelText('Upload files')
    const openPicker = vi
      .spyOn(fileInput, 'click')
      .mockImplementation(() => undefined)
    const trigger = screen.getByRole('button', { name: 'Attachments' })

    if (input === 'mouse') {
      await user.click(trigger)
      await user.click(
        await screen.findByRole('menuitem', { name: 'Add file' })
      )
    } else {
      trigger.focus()
      await user.keyboard('{ArrowDown}')
      const item = await screen.findByRole('menuitem', { name: 'Add file' })
      expect(item).toHaveFocus()
      await user.keyboard('{Enter}')
    }

    expect(openPicker).toHaveBeenCalledOnce()
  }
)
