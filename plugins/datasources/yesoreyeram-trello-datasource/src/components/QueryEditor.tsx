import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { InlineField, InlineFieldRow, InlineSwitch, Input, MultiSelect, RadioButtonGroup, Select } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  TrelloDataSourceOptions,
  TrelloQuery,
  TrelloQueryType,
  TrelloCardFilter,
  TrelloDateMode,
  CARD_FILTER_OPTIONS,
  CARD_FIELD_OPTIONS,
  DATE_MODE_OPTIONS,
  BoardInfo,
  ListInfo,
  MemberInfo,
  LabelInfo,
} from '../types';

type Props = QueryEditorProps<DataSource, TrelloQuery, TrelloDataSourceOptions>;

const LABEL_WIDTH = 20;
const INPUT_WIDTH = 40;
const DATE_LABEL_WIDTH = 8;
const DATE_INPUT_WIDTH = 18;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<TrelloQueryType>> = [
  { label: 'Cards', value: 'cards', description: 'List cards from a board or list' },
  { label: 'Count', value: 'count', description: 'Count cards on a board or list' },
];

const DATE_MODE_SELECT_OPTIONS: Array<SelectableValue<TrelloDateMode>> = DATE_MODE_OPTIONS.map((m) => ({
  label: m.label,
  value: m.value,
  description: m.description,
}));

function toMulti(values: string[] | undefined, options: Array<SelectableValue<string>>): Array<SelectableValue<string>> {
  return (values ?? []).map((v) => options.find((o) => o.value === v) ?? { label: v, value: v });
}

function multiValues(values: Array<SelectableValue<string>>): string[] {
  return values.map((v) => v.value).filter((v): v is string => v != null && v !== '');
}

function errMessage(err: unknown, fallback: string): string {
  const e = err as { data?: { error?: string }; message?: string };
  return e?.data?.error ?? e?.message ?? fallback;
}

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
  const { boardId, listId, memberIds, labelIds, fields, createdAfter, createdBefore, limit } = query;
  const queryType = query.queryType ?? 'cards';
  const cardFilter: TrelloCardFilter = query.cardFilter ?? 'all';
  const createdMode: TrelloDateMode = query.createdMode ?? 'any';

  const isCards = queryType === 'cards';

  const boards = useResource<BoardInfo>(
    () => datasource.getBoards(),
    (items) => items.map((b) => ({ label: b.name, value: b.id, description: b.desc })),
    true,
    [datasource]
  );

  const lists = useResource<ListInfo>(
    () => datasource.getLists(boardId ?? ''),
    (items) => items.filter((l) => !l.closed).map((l) => ({ label: l.name, value: l.id })),
    !!boardId,
    [datasource, boardId]
  );

  const members = useResource<MemberInfo>(
    () => datasource.getMembers(boardId ?? ''),
    (items) => items.map((m) => ({ label: m.fullName ?? m.username ?? m.id, value: m.id })),
    !!boardId,
    [datasource, boardId]
  );

  const labels = useResource<LabelInfo>(
    () => datasource.getLabels(boardId ?? ''),
    (items) => items.map((l) => ({ label: l.name || `(${l.color})`, value: l.id })),
    !!boardId,
    [datasource, boardId]
  );

  const update = useCallback(
    (patch: Partial<TrelloQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const updateAndRun = useCallback(
    (patch: Partial<TrelloQuery>) => {
      update(patch);
      onRunQuery();
    },
    [update, onRunQuery]
  );

  const onQueryTypeChange = (value: TrelloQueryType) => updateAndRun({ queryType: value });

  const onBoardChange = (value: SelectableValue<string> | null) =>
    updateAndRun({ boardId: value?.value ?? '', listId: '' });

  const onListChange = (value: SelectableValue<string> | null) => updateAndRun({ listId: value?.value ?? '' });

  const onCardFilterChange = (value: TrelloCardFilter) => updateAndRun({ cardFilter: value });

  const onCreatedAfter = (e: ChangeEvent<HTMLInputElement>) => update({ createdAfter: e.target.value });
  const onCreatedBefore = (e: ChangeEvent<HTMLInputElement>) => update({ createdBefore: e.target.value });
  const onCreatedModeChange = (mode: TrelloDateMode) =>
    updateAndRun({ createdMode: mode, ...(mode === 'custom' ? {} : { createdAfter: '', createdBefore: '' }) });

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const selected = (id: string | undefined, opts: Array<SelectableValue<string>>): SelectableValue<string> | null =>
    id ? opts.find((o) => o.value === id) ?? { label: id, value: id } : null;

  return (
    <div>
      <div className="gf-form">
        <InlineField label="Query type" labelWidth={LABEL_WIDTH} tooltip="What to fetch from Trello.">
          <RadioButtonGroup<TrelloQueryType> options={QUERY_TYPE_OPTIONS} value={queryType} onChange={onQueryTypeChange} />
        </InlineField>
      </div>

      <InlineFieldRow>
        <InlineField
          label="Board"
          labelWidth={LABEL_WIDTH}
          tooltip="The Trello board to query."
          error={boards.error}
          invalid={!!boards.error}
        >
          <Select<string>
            width={INPUT_WIDTH}
            isClearable
            isLoading={boards.loading}
            options={boards.options}
            value={selected(boardId, boards.options)}
            onChange={onBoardChange}
            allowCustomValue
            placeholder="Select a board"
            noOptionsMessage="No boards found"
          />
        </InlineField>
      </InlineFieldRow>

      {boardId && (
        <>
          <InlineFieldRow>
            <InlineField
              label="List"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by a specific list on the board (optional)."
              error={lists.error}
              invalid={!!lists.error}
            >
              <Select<string>
                width={INPUT_WIDTH}
                isClearable
                isLoading={lists.loading}
                options={lists.options}
                value={selected(listId, lists.options)}
                onChange={onListChange}
                allowCustomValue
                placeholder="All lists"
                noOptionsMessage="No lists found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Card filter" labelWidth={LABEL_WIDTH} tooltip="Filter cards by open/closed status.">
              <RadioButtonGroup<TrelloCardFilter> options={CARD_FILTER_OPTIONS} value={cardFilter} onChange={onCardFilterChange} />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Members"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter cards by one or more members (matches any)."
              error={members.error}
              invalid={!!members.error}
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                isLoading={members.loading}
                options={members.options}
                value={toMulti(memberIds, members.options)}
                onChange={(v) => updateAndRun({ memberIds: multiValues(v) })}
                allowCustomValue
                placeholder="Any member"
                noOptionsMessage="No members found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Labels"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter cards by one or more labels (matches any)."
              error={labels.error}
              invalid={!!labels.error}
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                isLoading={labels.loading}
                options={labels.options}
                value={toMulti(labelIds, labels.options)}
                onChange={(v) => updateAndRun({ labelIds: multiValues(v) })}
                allowCustomValue
                placeholder="Any label"
                noOptionsMessage="No labels found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Created"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter cards by creation date. 'Dashboard range' follows the panel's time picker; 'Custom' lets you enter explicit bounds. Trello derives creation date from the card id."
            >
              <RadioButtonGroup<TrelloDateMode> options={DATE_MODE_SELECT_OPTIONS} value={createdMode} onChange={onCreatedModeChange} />
            </InlineField>
            {createdMode === 'custom' && (
              <>
                <InlineField label="After" labelWidth={DATE_LABEL_WIDTH} tooltip="Lower bound. ISO-8601 (2024-01-01) or Unix millis.">
                  <Input width={DATE_INPUT_WIDTH} value={createdAfter ?? ''} placeholder="2024-01-01" onChange={onCreatedAfter} onBlur={onRunQuery} />
                </InlineField>
                <InlineField label="Before" labelWidth={DATE_LABEL_WIDTH} tooltip="Upper bound. ISO-8601 or Unix millis.">
                  <Input width={DATE_INPUT_WIDTH} value={createdBefore ?? ''} placeholder="2024-12-31" onChange={onCreatedBefore} onBlur={onRunQuery} />
                </InlineField>
              </>
            )}
          </InlineFieldRow>

          {isCards && (
            <>
              <InlineFieldRow>
                <InlineField label="Fields" labelWidth={LABEL_WIDTH} tooltip="Columns to return. Leave empty for all fields." grow>
                  <MultiSelect<string>
                    width={INPUT_WIDTH + 20}
                    options={CARD_FIELD_OPTIONS}
                    value={toMulti(fields, CARD_FIELD_OPTIONS)}
                    onChange={(v) => updateAndRun({ fields: multiValues(v) })}
                    allowCustomValue
                    placeholder="All fields"
                    noOptionsMessage="No fields available"
                  />
                </InlineField>
              </InlineFieldRow>

              <InlineFieldRow>
                <InlineField label="Limit" labelWidth={LABEL_WIDTH} tooltip="Maximum number of cards. 0 returns all (auto-paginated via the before cursor).">
                  <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
                </InlineField>
              </InlineFieldRow>
            </>
          )}
        </>
      )}
      <InlineFieldRow>
        <InlineField
          label="Hide system fields"
          labelWidth={LABEL_WIDTH}
          tooltip="Hide metadata-style columns (id, created_at/updated_at, underscore-prefixed names, etc.) from the returned frame."
        >
          <InlineSwitch
            value={!!query.hideSystemFields}
            onChange={(e) => {
              update({ hideSystemFields: e.currentTarget.checked });
              onRunQuery();
            }}
          />
        </InlineField>
      </InlineFieldRow>

    </div>
  );
}
