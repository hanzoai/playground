import React, { useState } from 'react';
import type { CreateTaskParams } from '../../services/tasksApi';

interface TaskSubmitDialogProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (task: CreateTaskParams) => Promise<void>;
}

const inputClass =
  'w-full rounded-md bg-[#0a0a0f] border border-white/10 text-white px-3 py-2 text-sm placeholder:text-white/30 focus:outline-none focus:ring-1 focus:ring-white/20';

const labelClass = 'block text-sm font-medium text-white/70 mb-1';

export default function TaskSubmitDialog({ open, onClose, onSubmit }: TaskSubmitDialogProps) {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [priority, setPriority] = useState<'low' | 'medium' | 'high' | 'critical'>('medium');
  const [inputPayload, setInputPayload] = useState('');
  const [targetBot, setTargetBot] = useState('');
  const [timeout, setTimeout] = useState(300);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!open) return null;

  const resetForm = () => {
    setTitle('');
    setDescription('');
    setPriority('medium');
    setInputPayload('');
    setTargetBot('');
    setTimeout(300);
    setError(null);
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!title.trim()) {
      setError('Title is required.');
      return;
    }

    let parsedInput: any = undefined;
    if (inputPayload.trim()) {
      try {
        parsedInput = JSON.parse(inputPayload);
      } catch {
        setError('Input payload must be valid JSON.');
        return;
      }
    }

    const params: CreateTaskParams = {
      title: title.trim(),
      ...(description.trim() ? { description: description.trim() } : {}),
      priority,
      ...(parsedInput !== undefined ? { input: parsedInput } : {}),
      ...(targetBot.trim() ? { target_bot: targetBot.trim() } : {}),
      timeout_ms: timeout * 1000,
    };

    setLoading(true);
    try {
      await onSubmit(params);
      resetForm();
      onClose();
    } catch (err: any) {
      setError(err?.message || 'Failed to submit task.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={handleClose}
    >
      <div
        className="w-full max-w-[500px] rounded-lg border border-white/10 bg-[#111118] p-6 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="mb-4 text-lg font-semibold text-white">Submit New Task</h2>

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Title */}
          <div>
            <label className={labelClass}>
              Title <span className="text-red-400">*</span>
            </label>
            <input
              type="text"
              className={inputClass}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Task title"
              autoFocus
            />
          </div>

          {/* Description */}
          <div>
            <label className={labelClass}>Description</label>
            <textarea
              className={`${inputClass} min-h-[72px] resize-y`}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional description"
              rows={3}
            />
          </div>

          {/* Priority */}
          <div>
            <label className={labelClass}>Priority</label>
            <select
              className={inputClass}
              value={priority}
              onChange={(e) => setPriority(e.target.value as any)}
            >
              <option value="low">Low</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
              <option value="critical">Critical</option>
            </select>
          </div>

          {/* Input Payload */}
          <div>
            <label className={labelClass}>Input Payload (JSON)</label>
            <textarea
              className={`${inputClass} min-h-[80px] resize-y font-mono text-xs`}
              value={inputPayload}
              onChange={(e) => setInputPayload(e.target.value)}
              placeholder='{"key": "value"}'
              rows={3}
            />
          </div>

          {/* Target Bot */}
          <div>
            <label className={labelClass}>Target Bot</label>
            <input
              type="text"
              className={inputClass}
              value={targetBot}
              onChange={(e) => setTargetBot(e.target.value)}
              placeholder="any available"
            />
          </div>

          {/* Timeout */}
          <div>
            <label className={labelClass}>Timeout (seconds)</label>
            <input
              type="number"
              className={inputClass}
              value={timeout}
              onChange={(e) => setTimeout(Number(e.target.value) || 0)}
              min={0}
              placeholder="300"
            />
          </div>

          {/* Error */}
          {error && (
            <p className="rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-400">
              {error}
            </p>
          )}

          {/* Buttons */}
          <div className="flex items-center justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={handleClose}
              disabled={loading}
              className="rounded-md border border-white/10 px-4 py-2 text-sm text-white/70 hover:bg-white/5 disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="rounded-md bg-white px-4 py-2 text-sm font-medium text-black hover:bg-white/90 disabled:opacity-50"
            >
              {loading ? 'Submitting...' : 'Submit Task'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
