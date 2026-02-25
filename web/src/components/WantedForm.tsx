import { useState, useEffect } from 'react';
import { toast } from 'sonner';
import { createItem, updateItem } from '../api/client';
import type { WantedItem } from '../api/types';
import { useFocusTrap } from '../hooks/useFocusTrap';
import styles from './WantedForm.module.css';

const types = ['feature', 'bug', 'design', 'rfc', 'docs'];
const priorities = [0, 1, 2, 3, 4];
const efforts = ['trivial', 'small', 'medium', 'large', 'epic'];

interface WantedFormProps {
  item?: WantedItem;
  onClose: () => void;
  onSaved: () => void;
}

export function WantedForm({ item, onClose, onSaved }: WantedFormProps) {
  const isEdit = !!item;

  const [title, setTitle] = useState(item?.title ?? '');
  const [description, setDescription] = useState(item?.description ?? '');
  const [project, setProject] = useState(item?.project ?? '');
  const [type, setType] = useState(item?.type ?? 'feature');
  const [priority, setPriority] = useState(item?.priority ?? 2);
  const [effortLevel, setEffortLevel] = useState(item?.effort_level ?? 'medium');
  const [tags, setTags] = useState(item?.tags?.join(', ') ?? '');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') handleSubmit();
    };
    window.addEventListener('keydown', handleKey);
    return () => window.removeEventListener('keydown', handleKey);
  });

  const handleSubmit = async () => {
    if (!title.trim() || saving) return;
    setSaving(true);

    const parsedTags = tags
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);

    try {
      if (isEdit && item) {
        await updateItem(item.id, {
          title: title.trim(),
          description: description.trim() || undefined,
          project: project.trim() || undefined,
          type,
          priority,
          effort_level: effortLevel,
          tags: parsedTags,
          tags_set: true,
        });
        toast.success('Item updated');
      } else {
        await createItem({
          title: title.trim(),
          description: description.trim() || undefined,
          project: project.trim() || undefined,
          type,
          priority,
          effort_level: effortLevel,
          tags: parsedTags.length > 0 ? parsedTags : undefined,
        });
        toast.success('Item posted');
      }
      onSaved();
      onClose();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  const trapRef = useFocusTrap(true);

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div
        ref={trapRef}
        className={styles.dialog}
        role="dialog"
        aria-modal="true"
        aria-label={isEdit ? 'Edit item' : 'Post new item'}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className={styles.title}>{isEdit ? 'Edit Item' : 'Post New Item'}</h2>

        <div className={styles.field}>
          <label className={styles.label}>Title</label>
          <input
            className={styles.input}
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="What needs to be done?"
            autoFocus
          />
        </div>

        <div className={styles.field}>
          <label className={styles.label}>Description</label>
          <textarea
            className={styles.textarea}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Details, context, acceptance criteria..."
          />
        </div>

        <div className={styles.row}>
          <div className={styles.field}>
            <label className={styles.label}>Project</label>
            <input
              className={styles.input}
              type="text"
              value={project}
              onChange={(e) => setProject(e.target.value)}
              placeholder="project name"
            />
          </div>
          <div className={styles.field}>
            <label className={styles.label}>Type</label>
            <select
              className={styles.select}
              value={type}
              onChange={(e) => setType(e.target.value)}
            >
              {types.map((t) => (
                <option key={t} value={t}>{t}</option>
              ))}
            </select>
          </div>
        </div>

        <div className={styles.row}>
          <div className={styles.field}>
            <label className={styles.label}>Priority</label>
            <select
              className={styles.select}
              value={priority}
              onChange={(e) => setPriority(Number(e.target.value))}
            >
              {priorities.map((p) => (
                <option key={p} value={p}>P{p}</option>
              ))}
            </select>
          </div>
          <div className={styles.field}>
            <label className={styles.label}>Effort</label>
            <select
              className={styles.select}
              value={effortLevel}
              onChange={(e) => setEffortLevel(e.target.value)}
            >
              {efforts.map((e) => (
                <option key={e} value={e}>{e}</option>
              ))}
            </select>
          </div>
        </div>

        <div className={styles.field}>
          <label className={styles.label}>Tags</label>
          <input
            className={styles.input}
            type="text"
            value={tags}
            onChange={(e) => setTags(e.target.value)}
            placeholder="tag1, tag2, ..."
          />
        </div>

        <div className={styles.actions}>
          <button className={styles.cancelBtn} onClick={onClose}>
            Cancel
          </button>
          <button
            className={styles.submitBtn}
            onClick={handleSubmit}
            disabled={!title.trim() || saving}
          >
            {saving ? 'Saving...' : isEdit ? 'Update' : 'Post'}
          </button>
        </div>

        <p className={styles.hint}>Cmd+Enter to submit &middot; Esc to close</p>
      </div>
    </div>
  );
}
