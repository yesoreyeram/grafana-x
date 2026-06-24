import React from 'react';
import { Select, Input, Button, IconButton, InlineFieldRow, InlineLabel, useStyles2 } from '@grafana/ui';
import { GrafanaTheme2, SelectableValue } from '@grafana/data';
import { css } from '@emotion/css';

import {
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

const CONNECTOR_WIDTH = 10;
const FIELD_WIDTH = 22;
const OPERATOR_WIDTH = 18;
const VALUE_WIDTH = 24;

interface FilterEditorProps {
  group: FilterGroup;
  disabled?: boolean;
  onChange: (group: FilterGroup) => void;
}

/** Top-level filter builder. The root is always a group. */
export function FilterEditor({ group, disabled, onChange }: FilterEditorProps) {
  return <GroupEditor group={group} disabled={disabled} depth={0} onChange={onChange} />;
}

interface GroupEditorProps extends FilterEditorProps {
  depth: number;
}

function GroupEditor({ group, disabled, depth, onChange }: GroupEditorProps) {
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
  disabled?: boolean;
  onChange: (c: FilterCondition) => void;
  onRemove: () => void;
}

function ConditionRow({ condition, disabled, onChange, onRemove }: ConditionRowProps) {
  const opOptions = operatorOptions();
  const arity = operatorArity(condition.op);

  return (
    <>
      <Input
        width={FIELD_WIDTH}
        placeholder="Column name"
        value={condition.field}
        onChange={(e) => onChange({ ...condition, field: e.currentTarget.value })}
        disabled={disabled}
      />
      <Select<string>
        width={OPERATOR_WIDTH}
        options={opOptions}
        value={opOptions.find((o) => o.value === condition.op) ?? { label: condition.op, value: condition.op }}
        onChange={(v) => onChange({ ...condition, op: v?.value ?? 'eq' })}
        disabled={disabled || !condition.field}
        placeholder="Operator"
      />
      {arity === 'none' ? (
        <span style={{ display: 'inline-block', width: VALUE_WIDTH * 8 }} />
      ) : (
        <Input
          width={VALUE_WIDTH}
          value={condition.value ?? ''}
          placeholder="Value"
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
    marginBottom: theme.spacing(0.5),
    alignItems: 'center',
  }),
  connectorLabel: css({
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
});
