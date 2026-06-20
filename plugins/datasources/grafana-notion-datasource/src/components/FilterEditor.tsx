import React from 'react';
import { Select, Input, Button, IconButton, InlineFieldRow, InlineLabel, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2, SelectableValue } from '@grafana/data';
import { css } from '@emotion/css';

import { PropertyInfo } from '../types';
import {
  categoryForType,
  FilterCondition,
  FilterGroup,
  FilterNode,
  LogicalConnector,
  newCondition,
  newGroup,
  operatorArity,
  operatorOptions,
} from '../filter';

const CONNECTOR_OPTIONS: Array<SelectableValue<LogicalConnector>> = [
  { label: 'AND', value: 'and' },
  { label: 'OR', value: 'or' },
];

// Notion stores checkbox values as true/false.
const BOOL_OPTIONS: Array<SelectableValue<string>> = [
  { label: 'Checked', value: 'true' },
  { label: 'Unchecked', value: 'false' },
];

// Control widths (Grafana units = 8px each). Kept consistent across all rows so
// the field / operator / value columns line up, and aligned with the rest of the
// query editor's controls.
const CONNECTOR_WIDTH = 10;
const FIELD_WIDTH = 22;
const OPERATOR_WIDTH = 18;
const VALUE_WIDTH = 24;

interface FilterEditorProps {
  group: FilterGroup;
  fields: PropertyInfo[];
  disabled?: boolean;
  onChange: (group: FilterGroup) => void;
}

/** Top-level filter builder. The root is always a group. */
export function FilterEditor({ group, fields, disabled, onChange }: FilterEditorProps) {
  return <GroupEditor group={group} fields={fields} disabled={disabled} depth={0} onChange={onChange} />;
}

interface GroupEditorProps extends FilterEditorProps {
  depth: number;
}

function GroupEditor({ group, fields, disabled, depth, onChange }: GroupEditorProps) {
  const styles = useStyles2(getStyles);

  const updateChild = (index: number, child: FilterNode) => {
    onChange({ ...group, children: group.children.map((c, i) => (i === index ? child : c)) });
  };

  const removeChild = (index: number) => {
    onChange({ ...group, children: group.children.filter((_, i) => i !== index) });
  };

  const addCondition = () => {
    onChange({ ...group, children: [...group.children, newCondition()] });
  };

  const addGroup = () => {
    onChange({ ...group, children: [...group.children, { ...newGroup(), children: [newCondition()] }] });
  };

  const setConnector = (connector: LogicalConnector) => {
    onChange({ ...group, connector });
  };

  return (
    <div className={depth > 0 ? styles.nested : undefined}>
      {group.children.map((child, index) => {
        const connectorCell =
          index === 0 ? (
            <InlineLabel width={CONNECTOR_WIDTH} className={styles.connectorLabel}>
              Where
            </InlineLabel>
          ) : (
            <Select<LogicalConnector>
              width={CONNECTOR_WIDTH}
              options={CONNECTOR_OPTIONS}
              value={CONNECTOR_OPTIONS.find((o) => o.value === group.connector)}
              onChange={(v) => v?.value && setConnector(v.value)}
              disabled={disabled}
              aria-label="Filter connector"
            />
          );

        if (child.kind === 'condition') {
          return (
            <InlineFieldRow key={index} className={styles.row}>
              {connectorCell}
              <ConditionRow
                condition={child}
                fields={fields}
                disabled={disabled}
                onChange={(c) => updateChild(index, c)}
                onRemove={() => removeChild(index)}
              />
            </InlineFieldRow>
          );
        }

        return (
          <InlineFieldRow key={index} className={styles.row} style={{ alignItems: 'flex-start' }}>
            {connectorCell}
            <div className={styles.groupCard}>
              <div className={styles.groupHeader}>
                <span className={styles.groupTitle}>Group</span>
                <IconButton
                  name="trash-alt"
                  tooltip="Remove group"
                  aria-label="Remove group"
                  onClick={() => removeChild(index)}
                />
              </div>
              <GroupEditor
                group={child}
                fields={fields}
                disabled={disabled}
                depth={depth + 1}
                onChange={(g) => updateChild(index, g)}
              />
            </div>
          </InlineFieldRow>
        );
      })}
      <InlineFieldRow className={styles.row}>
        <Button variant="secondary" size="sm" icon="plus" onClick={addCondition} disabled={disabled}>
          Add filter
        </Button>
        <Button variant="secondary" size="sm" icon="plus" onClick={addGroup} disabled={disabled}>
          Add filter group
        </Button>
      </InlineFieldRow>
    </div>
  );
}

interface ConditionRowProps {
  condition: FilterCondition;
  fields: PropertyInfo[];
  disabled?: boolean;
  onChange: (c: FilterCondition) => void;
  onRemove: () => void;
}

function ConditionRow({ condition, fields, disabled, onChange, onRemove }: ConditionRowProps) {
  const fieldOptions: Array<SelectableValue<string>> = fields.map((f) => ({
    label: f.title,
    value: f.title,
    description: f.type,
  }));

  const selectedFieldType = fields.find((f) => f.title === condition.field)?.type;
  const opOptions = operatorOptions(selectedFieldType);
  const arity = operatorArity(condition.op);
  const isBoolean = categoryForType(selectedFieldType) === 'checkbox';

  const onFieldChange = (v: SelectableValue<string> | null) => {
    const field = v?.value ?? '';
    const newType = fields.find((f) => f.title === field)?.type;
    const category = categoryForType(newType);
    const validOps = operatorOptions(newType);
    // Keep current op if still valid for the new field type, else default to first.
    const op = validOps.some((o) => o.value === condition.op) ? condition.op : validOps[0]?.value ?? 'equals';
    onChange({ ...condition, field, category, op });
  };

  return (
    <>
      <Select<string>
        width={FIELD_WIDTH}
        options={fieldOptions}
        value={
          condition.field
            ? fieldOptions.find((o) => o.value === condition.field) ?? { label: condition.field, value: condition.field }
            : null
        }
        onChange={onFieldChange}
        allowCustomValue
        placeholder="Property"
        disabled={disabled}
        noOptionsMessage="No properties"
      />
      <Select<string>
        width={OPERATOR_WIDTH}
        options={opOptions}
        value={opOptions.find((o) => o.value === condition.op) ?? { label: condition.op, value: condition.op }}
        onChange={(v) => onChange({ ...condition, op: v?.value ?? 'equals' })}
        disabled={disabled || !condition.field}
        placeholder="Operator"
      />
      {arity === 'none' ? (
        // Reserve the value column width so the remove button stays aligned.
        <span style={{ display: 'inline-block', width: VALUE_WIDTH * 8 }} />
      ) : arity === 'single' && isBoolean ? (
        <Select<string>
          width={VALUE_WIDTH}
          options={BOOL_OPTIONS}
          value={BOOL_OPTIONS.find((o) => o.value === condition.value) ?? null}
          onChange={(v) => onChange({ ...condition, value: v?.value ?? '' })}
          placeholder="Value"
          disabled={disabled}
        />
      ) : (
        <Input
          width={VALUE_WIDTH}
          value={condition.value ?? ''}
          placeholder={arity === 'list' ? 'value1, value2, …' : 'Value'}
          onChange={(e) => onChange({ ...condition, value: e.currentTarget.value })}
          disabled={disabled}
        />
      )}
      <IconButton name="trash-alt" tooltip="Remove filter" aria-label="Remove filter" onClick={onRemove} />
    </>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  row: css({
    // Match the spacing rhythm of the rest of the query editor rows.
    marginBottom: theme.spacing(0.5),
    alignItems: 'center',
  }),
  connectorLabel: css({
    // Use the same muted look as InlineLabel but make "Where" read as a label.
    justifyContent: 'flex-start',
  }),
  nested: css({
    minWidth: 0,
  }),
  groupCard: css({
    minWidth: 0,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    padding: theme.spacing(1),
    background: theme.colors.background.secondary,
  }),
  groupHeader: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: theme.spacing(0.5),
  }),
  groupTitle: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.bodySmall.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    textTransform: 'uppercase',
    letterSpacing: '0.02em',
  }),
  emptyHint: css({
    color: theme.colors.text.secondary,
    fontStyle: 'italic',
    marginBottom: theme.spacing(1),
  }),
});
