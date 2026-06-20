import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import {
  InlineField,
  InlineFieldRow,
  Input,
  MultiSelect,
  RadioButtonGroup,
  Select,
  TextArea,
  InlineSwitch,
} from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  AGGREGATION_OPTIONS,
  AGGREGATIONS_NEEDING_COLUMN,
  BoardInfo,
  ColumnInfo,
  GroupInfo,
  MondayAggregation,
  MondayDataSourceOptions,
  MondayOrderDir,
  MondayQuery,
  MondayQueryType,
  MondayState,
  ORDER_DIR_OPTIONS,
  STATE_OPTIONS,
  WorkspaceInfo,
} from '../types';

type Props = QueryEditorProps<DataSource, MondayQuery, MondayDataSourceOptions>;

const LABEL_WIDTH = 26;
const INPUT_WIDTH = 40;
const WIDE_WIDTH = 60;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<MondayQueryType>> = [
  { label: 'Items', value: 'items', description: 'List items from one or more boards' },
  { label: 'Boards', value: 'boards', description: 'List boards' },
  { label: 'Groups', value: 'groups', description: 'List the groups of a board' },
  { label: 'Users', value: 'users', description: 'List account users' },
  { label: 'Workspaces', value: 'workspaces', description: 'List workspaces' },
  { label: 'Tags', value: 'tags', description: 'List account tags' },
  { label: 'Raw GraphQL', value: 'raw', description: 'Run a custom GraphQL query' },
];

const STATE_SELECT_OPTIONS: Array<SelectableValue<MondayState>> = STATE_OPTIONS.map((s) => ({
  label: s.label,
  value: s.value,
  description: s.description,
}));

const ORDER_DIR_SELECT_OPTIONS: Array<SelectableValue<MondayOrderDir>> = ORDER_DIR_OPTIONS.map((o) => ({
  label: o.label,
  value: o.value,
}));

const AGGREGATION_SELECT_OPTIONS: Array<SelectableValue<MondayAggregation>> = AGGREGATION_OPTIONS.map((a) => ({
  label: a.label,
  value: a.value,
  description: a.description,
}));

// monday's server-side aggregate API groups by board column ID, so the
// group-by / value pickers offer column ids (labelled by title).
function columnFieldOptions(columns: ColumnInfo[]): Array<SelectableValue<string>> {
  return columns.map((c) => ({
    label: c.title,
    value: c.id,
    description: c.type ?? c.id,
  }));
}

const RAW_PLACEHOLDER = `query {
  boards(ids: [1234567890]) {
    items_page(limit: 50) {
      items { id name created_at }
    }
  }
}`;

/** Map a stored string[] value to selected options, preserving custom values. */
function toMulti(
  values: string[] | undefined,
  options: Array<SelectableValue<string>>
): Array<SelectableValue<string>> {
  return (values ?? []).map((v) => options.find((o) => o.value === v) ?? { label: v, value: v });
}

/** Extract plain string values from selected options. */
function multiValues(values: Array<SelectableValue<string>>): string[] {
  return values.map((v) => v.value).filter((v): v is string => v != null && v !== '');
}

/** Read a resource-call error into a short message. */
function errMessage(err: unknown, fallback: string): string {
  const e = err as { data?: { error?: string }; message?: string };
  return e?.data?.error ?? e?.message ?? fallback;
}

/**
 * useResource loads a list via the provided loader whenever `enabled` is true and
 * any dependency changes. Each list is loaded independently so one failure never
 * blanks the others. Returns the options, a loading flag, and an error message.
 */
function useResource<T>(
  loader: () => Promise<T[]>,
  toOptions: (items: T[]) => Array<SelectableValue<string>>,
  enabled: boolean,
  deps: React.DependencyList
): { options: Array<SelectableValue<string>>; loading: boolean; error?: string } {
  const [options, setOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | undefined>();

  useEffect(() => {
    if (!enabled) {
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(undefined);
    loader()
      .then((items) => {
        if (!cancelled) {
          setOptions(toOptions(items));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setOptions([]);
          setError(errMessage(err, 'Failed to load'));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { options, loading, error };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const {
    boardIds,
    groupIds,
    workspaceIds,
    columnIds,
    searchQuery,
    includeColumnValues,
    hideSystemColumns,
    orderBy,
    groupBy,
    aggregationColumn,
    rawQuery,
    rawVariables,
    limit,
  } = query;
  const queryType = query.queryType ?? 'items';
  const state: MondayState = query.state ?? 'active';
  const orderDir: MondayOrderDir = query.orderDir ?? 'asc';
  const aggregation: MondayAggregation = query.aggregation ?? 'count';

  const isRaw = queryType === 'raw';
  const isItems = queryType === 'items';
  const isBoards = queryType === 'boards';
  const isGroups = queryType === 'groups';
  const needsBoards = isItems || isGroups;
  const hasState = isItems || isBoards || queryType === 'workspaces';

  // Each list loads independently. A failure in one does not affect the others.
  const boards = useResource<BoardInfo>(
    () => datasource.getBoards(),
    (items) => items.map((b) => ({ label: b.name, value: b.id, description: b.id })),
    needsBoards || isBoards,
    [datasource, needsBoards, isBoards]
  );

  const groupList = useResource<GroupInfo>(
    () => datasource.getGroups(boardIds),
    (items) => items.map((g) => ({ label: g.title, value: g.id })),
    isItems && !!boardIds && boardIds.length > 0,
    [datasource, isItems, boardIds]
  );

  const columnList = useResource<ColumnInfo>(
    () => datasource.getColumns(boardIds),
    (items) => items.map((c) => ({ label: c.title, value: c.id, description: c.type })),
    isItems && !!boardIds && boardIds.length > 0,
    [datasource, isItems, boardIds]
  );

  // Raw column metadata, used to build the group-by / value-column pickers
  // (which key on the column title, not its id, since titles are the flattened
  // column names).
  const [rawColumns, setRawColumns] = useState<ColumnInfo[]>([]);
  useEffect(() => {
    if (!isItems || !boardIds || boardIds.length === 0) {
      setRawColumns([]);
      return;
    }
    let cancelled = false;
    datasource
      .getColumns(boardIds)
      .then((cols) => {
        if (!cancelled) {
          setRawColumns(cols);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setRawColumns([]);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [datasource, isItems, boardIds]);

  const groupFieldOptions = columnFieldOptions(rawColumns);

  const workspaceList = useResource<WorkspaceInfo>(
    () => datasource.getWorkspaces(),
    (items) => items.map((w) => ({ label: w.name, value: w.id, description: w.id })),
    isBoards,
    [datasource, isBoards]
  );

  const update = useCallback(
    (patch: Partial<MondayQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const updateAndRun = useCallback(
    (patch: Partial<MondayQuery>) => {
      update(patch);
      onRunQuery();
    },
    [update, onRunQuery]
  );

  const onQueryTypeChange = (value: MondayQueryType) => updateAndRun({ queryType: value });

  const onBoardsChange = (v: Array<SelectableValue<string>>) => {
    // Changing boards invalidates board-scoped selections (groups & columns).
    updateAndRun({ boardIds: multiValues(v), groupIds: [], columnIds: [] });
  };

  const onSearchChange = (e: ChangeEvent<HTMLInputElement>) => update({ searchQuery: e.target.value });
  const onOrderByChange = (e: ChangeEvent<HTMLInputElement>) => update({ orderBy: e.target.value });

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const onRawQueryChange = (e: ChangeEvent<HTMLTextAreaElement>) => update({ rawQuery: e.target.value });
  const onRawVariablesChange = (e: ChangeEvent<HTMLTextAreaElement>) => update({ rawVariables: e.target.value });

  const onGroupByChange = (v: SelectableValue<string> | null) =>
    updateAndRun({ groupBy: v?.value ?? '' });
  const onAggregationChange = (v: MondayAggregation) => updateAndRun({ aggregation: v });
  const onAggregationColumnChange = (v: SelectableValue<string> | null) =>
    updateAndRun({ aggregationColumn: v?.value ?? '' });

  const includeColumns = includeColumnValues ?? true;
  const isGrouped = !!groupBy && groupBy.trim().length > 0;
  const aggNeedsColumn = AGGREGATIONS_NEEDING_COLUMN.includes(aggregation);

  const selectedGroupBy: SelectableValue<string> | null = groupBy
    ? groupFieldOptions.find((o) => o.value === groupBy) ?? { label: groupBy, value: groupBy }
    : null;
  const selectedAggColumn: SelectableValue<string> | null = aggregationColumn
    ? groupFieldOptions.find((o) => o.value === aggregationColumn) ?? {
        label: aggregationColumn,
        value: aggregationColumn,
      }
    : null;

  return (
    <div>
      <div className="gf-form">
        <InlineField label="Query type" labelWidth={LABEL_WIDTH} tooltip="What to fetch from monday.com.">
          <RadioButtonGroup<MondayQueryType>
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </div>

      {needsBoards && (
        <InlineFieldRow>
          <InlineField
            label="Boards"
            labelWidth={LABEL_WIDTH}
            tooltip="Boards to query. Required for items and groups."
            error={boards.error}
            invalid={!!boards.error}
          >
            <MultiSelect<string>
              width={WIDE_WIDTH}
              isLoading={boards.loading}
              options={boards.options}
              value={toMulti(boardIds, boards.options)}
              onChange={onBoardsChange}
              allowCustomValue
              placeholder="Select board(s)"
              noOptionsMessage="No boards found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {isBoards && (
        <InlineFieldRow>
          <InlineField
            label="Workspaces"
            labelWidth={LABEL_WIDTH}
            tooltip="Restrict boards to one or more workspaces. Leave empty for all."
            error={workspaceList.error}
            invalid={!!workspaceList.error}
          >
            <MultiSelect<string>
              width={INPUT_WIDTH}
              isLoading={workspaceList.loading}
              options={workspaceList.options}
              value={toMulti(workspaceIds, workspaceList.options)}
              onChange={(v) => updateAndRun({ workspaceIds: multiValues(v) })}
              allowCustomValue
              placeholder="All workspaces"
              noOptionsMessage="No workspaces found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {isItems && (
        <>
          <InlineFieldRow>
            <InlineField
              label="Groups"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter items to one or more groups within the selected board(s)."
              error={groupList.error}
              invalid={!!groupList.error}
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                isLoading={groupList.loading}
                options={groupList.options}
                value={toMulti(groupIds, groupList.options)}
                onChange={(v) => updateAndRun({ groupIds: multiValues(v) })}
                allowCustomValue
                placeholder="Any group"
                noOptionsMessage="No groups found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Columns"
              labelWidth={LABEL_WIDTH}
              tooltip="Restrict the returned column values to specific columns. Leave empty for all columns."
              error={columnList.error}
              invalid={!!columnList.error}
            >
              <MultiSelect<string>
                width={WIDE_WIDTH}
                isLoading={columnList.loading}
                options={columnList.options}
                value={toMulti(columnIds, columnList.options)}
                onChange={(v) => updateAndRun({ columnIds: multiValues(v) })}
                allowCustomValue
                placeholder="All columns"
                noOptionsMessage="No columns found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Name contains" labelWidth={LABEL_WIDTH} tooltip="Filter items by text in their name.">
              <Input
                width={INPUT_WIDTH}
                value={searchQuery ?? ''}
                placeholder="search name"
                onChange={onSearchChange}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Include column values"
              labelWidth={LABEL_WIDTH}
              tooltip="Flatten each item's column values into table columns. Disable for just the core item fields."
            >
              <InlineSwitch
                value={includeColumns}
                onChange={(e) => updateAndRun({ includeColumnValues: e.currentTarget.checked })}
              />
            </InlineField>
            {includeColumns && (
              <InlineField
                label="Hide system columns"
                labelWidth={LABEL_WIDTH}
                tooltip="Omit monday.com's built-in/system columns (subitems, last updated, creation log, formula, etc.)."
              >
                <InlineSwitch
                  value={!!hideSystemColumns}
                  onChange={(e) => updateAndRun({ hideSystemColumns: e.currentTarget.checked })}
                />
              </InlineField>
            )}
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Order by column"
              labelWidth={LABEL_WIDTH}
              tooltip="Column ID to order items by. Leave empty for monday's default order."
            >
              <Input
                width={INPUT_WIDTH}
                value={orderBy ?? ''}
                placeholder="e.g. status"
                onChange={onOrderByChange}
                onBlur={onRunQuery}
              />
            </InlineField>
            <InlineField label="Direction" labelWidth={12} tooltip="Order direction.">
              <RadioButtonGroup<MondayOrderDir>
                options={ORDER_DIR_SELECT_OPTIONS}
                value={orderDir}
                onChange={(v) => updateAndRun({ orderDir: v })}
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Group by"
              labelWidth={LABEL_WIDTH}
              tooltip="Aggregate items by a board column (e.g. status, owner) using monday's server-side aggregate API. Leave empty to return individual items."
            >
              <Select<string>
                width={INPUT_WIDTH}
                isClearable
                options={groupFieldOptions}
                value={selectedGroupBy}
                onChange={onGroupByChange}
                allowCustomValue
                placeholder="No grouping"
                noOptionsMessage="No columns found"
              />
            </InlineField>
            {isGrouped && (
              <InlineField label="Aggregation" labelWidth={14} tooltip="How to aggregate items within each group.">
                <Select<MondayAggregation>
                  width={20}
                  options={AGGREGATION_SELECT_OPTIONS}
                  value={aggregation}
                  onChange={(v) => v?.value && onAggregationChange(v.value)}
                />
              </InlineField>
            )}
          </InlineFieldRow>

          {isGrouped && aggNeedsColumn && (
            <InlineFieldRow>
              <InlineField
                label="Value column"
                labelWidth={LABEL_WIDTH}
                tooltip="Column whose values are aggregated (summed/averaged/min/max, or counted distinct)."
              >
                <Select<string>
                  width={INPUT_WIDTH}
                  isClearable
                  options={groupFieldOptions}
                  value={selectedAggColumn}
                  onChange={onAggregationColumnChange}
                  allowCustomValue
                  placeholder="Select column"
                  noOptionsMessage="No columns found"
                />
              </InlineField>
            </InlineFieldRow>
          )}
        </>
      )}

      {isRaw && (
        <>
          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField
              label="GraphQL"
              labelWidth={LABEL_WIDTH}
              tooltip="A monday.com GraphQL query. The first array of objects in the response is flattened into a table."
              grow
            >
              <TextArea
                rows={10}
                value={rawQuery ?? ''}
                placeholder={RAW_PLACEHOLDER}
                onChange={onRawQueryChange}
                onBlur={onRunQuery}
              />
            </InlineField>
          </div>
          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField
              label="Variables"
              labelWidth={LABEL_WIDTH}
              tooltip="Optional JSON object of GraphQL variables for the query above."
              grow
            >
              <TextArea
                rows={3}
                value={rawVariables ?? ''}
                placeholder={'{ "limit": 50 }'}
                onChange={onRawVariablesChange}
                onBlur={onRunQuery}
              />
            </InlineField>
          </div>
        </>
      )}

      {!isRaw && (
        <InlineFieldRow>
          {hasState && (
            <InlineField label="State" labelWidth={LABEL_WIDTH} tooltip="Lifecycle state to include.">
              <RadioButtonGroup<MondayState>
                options={STATE_SELECT_OPTIONS}
                value={state}
                onChange={(v) => updateAndRun({ state: v })}
              />
            </InlineField>
          )}
          <InlineField
            label="Limit"
            labelWidth={12}
            tooltip="Maximum number of records. 0 returns all (auto-paginated)."
          >
            <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
          </InlineField>
        </InlineFieldRow>
      )}
    </div>
  );
}
