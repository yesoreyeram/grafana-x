import React, { useCallback, useEffect, useState, type ChangeEvent } from 'react';
import { InlineField, InlineFieldRow, Input, RadioButtonGroup, Select } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  IntercomQuery,
  IntercomDataSourceOptions,
  IntercomQueryType,
  IntercomCountEntity,
  AdminInfo,
  TeamInfo,
  TagInfo,
  QUERY_TYPE_OPTIONS,
  COUNT_OF_OPTIONS,
  CONVERSATION_STATE_OPTIONS,
  CONTACT_ROLE_OPTIONS,
  DEFAULT_QUERY,
} from '../types';
import { SearchFilter, newFilter, operatorOptions } from '../filter';
import { parseSort, serializeSort, SortDirection } from '../sort';

type Props = QueryEditorProps<DataSource, IntercomQuery, IntercomDataSourceOptions>;

const LABEL_WIDTH = 22;
const INPUT_WIDTH = 40;

const SORT_DIR_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Ascending', value: 'asc' },
  { label: 'Descending', value: 'desc' },
];

const SEARCHABLE_ENTITIES = ['conversations', 'contacts', 'tickets'];
const PAGINATED_ENTITIES = ['conversations', 'contacts', 'tickets', 'articles', 'companies'];

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
  const { countOf, statusFilter, role, assigneeId, teamId, tagId, searchQuery, filters, sort, limit } = query;
  const queryType: IntercomQueryType = query.queryType ?? DEFAULT_QUERY.queryType!;

  const effectiveEntity = queryType === 'count' ? countOf ?? 'conversations' : queryType;
  const isSearchable = SEARCHABLE_ENTITIES.includes(effectiveEntity);
  const showStatus = effectiveEntity === 'conversations';
  const showRole = effectiveEntity === 'contacts';
  const showAssigneeTeam = effectiveEntity === 'conversations' || effectiveEntity === 'tickets';
  const showSortLimit = queryType !== 'count' && PAGINATED_ENTITIES.includes(effectiveEntity);

  const adminsResource = useResource<AdminInfo>(
    () => datasource.getAdmins(),
    (admins) => admins.map((a) => ({ label: a.name || a.id, value: a.id, description: a.email })),
    showAssigneeTeam,
    [datasource, effectiveEntity]
  );
  const teamsResource = useResource<TeamInfo>(
    () => datasource.getTeams(),
    (teams) => teams.map((t) => ({ label: t.name || t.id, value: t.id })),
    showAssigneeTeam,
    [datasource, effectiveEntity]
  );
  const tagsResource = useResource<TagInfo>(
    () => datasource.getTags(),
    (tags) => tags.map((t) => ({ label: t.name || t.id, value: t.id })),
    isSearchable,
    [datasource, effectiveEntity]
  );

  const update = useCallback((patch: Partial<IntercomQuery>) => onChange({ ...query, ...patch }), [onChange, query]);
  const updateAndRun = useCallback(
    (patch: Partial<IntercomQuery>) => {
      onChange({ ...query, ...patch });
      onRunQuery();
    },
    [onChange, query, onRunQuery]
  );

  const onInputChange = (field: keyof IntercomQuery) => (e: ChangeEvent<HTMLInputElement>) => {
    update({ [field]: e.target.value } as Partial<IntercomQuery>);
  };

  // Filter row management.
  const currentFilters: SearchFilter[] = filters ?? [];
  const addFilter = () => update({ filters: [...currentFilters, newFilter()] });
  const removeFilter = (idx: number) => {
    const next = [...currentFilters];
    next.splice(idx, 1);
    updateAndRun({ filters: next });
  };
  const updateFilter = (idx: number, patch: Partial<SearchFilter>) => {
    const next = currentFilters.map((f, i) => (i === idx ? { ...f, ...patch } : f));
    update({ filters: next });
  };

  // Sort helpers.
  const sortItem = parseSort(sort);
  const sortField = sortItem?.field ?? '';
  const sortDir: SortDirection = sortItem?.direction ?? 'desc';
  const onSortFieldChange = (e: ChangeEvent<HTMLInputElement>) => {
    update({ sort: serializeSort({ field: e.target.value, direction: sortDir }) });
  };
  const onSortDirChange = (dir: SortDirection) => {
    updateAndRun({ sort: serializeSort({ field: sortField, direction: dir }) });
  };

  const selectedAssignee = assigneeId
    ? adminsResource.options.find((o) => o.value === assigneeId) ?? { label: assigneeId, value: assigneeId }
    : null;
  const selectedTeam = teamId
    ? teamsResource.options.find((o) => o.value === teamId) ?? { label: teamId, value: teamId }
    : null;
  const selectedTag = tagId
    ? tagsResource.options.find((o) => o.value === tagId) ?? { label: tagId, value: tagId }
    : null;

  return (
    <div>
      <InlineField label="Query type" labelWidth={LABEL_WIDTH} tooltip="Intercom entity to query. Conversations/Contacts list by default and switch to search when filters are set; Tickets always use search.">
        <Select<string>
          width={INPUT_WIDTH}
          options={QUERY_TYPE_OPTIONS}
          value={queryType}
          onChange={(v) => v?.value && updateAndRun({ queryType: v.value as IntercomQueryType })}
          placeholder="Select query type"
        />
      </InlineField>

      {queryType === 'count' && (
        <InlineField label="Count of" labelWidth={LABEL_WIDTH} tooltip="Entity to count. Uses the API's total_count.">
          <Select<string>
            width={INPUT_WIDTH}
            options={COUNT_OF_OPTIONS}
            value={countOf ?? 'conversations'}
            onChange={(v) => v?.value && updateAndRun({ countOf: v.value as IntercomCountEntity })}
          />
        </InlineField>
      )}

      {showStatus && (
        <InlineField label="State" labelWidth={LABEL_WIDTH} tooltip="Conversation state filter.">
          <RadioButtonGroup<string>
            options={CONVERSATION_STATE_OPTIONS}
            value={statusFilter ?? ''}
            onChange={(v) => updateAndRun({ statusFilter: v })}
          />
        </InlineField>
      )}

      {showRole && (
        <InlineField label="Role" labelWidth={LABEL_WIDTH} tooltip="Contact role filter (user or lead).">
          <RadioButtonGroup<string>
            options={CONTACT_ROLE_OPTIONS}
            value={role ?? ''}
            onChange={(v) => updateAndRun({ role: v })}
          />
        </InlineField>
      )}

      {showAssigneeTeam && (
        <InlineFieldRow>
          <InlineField
            label="Assignee"
            labelWidth={LABEL_WIDTH}
            tooltip="Filter by admin assignee (admin_assignee_id)."
            error={adminsResource.error}
            invalid={!!adminsResource.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={adminsResource.loading}
              options={adminsResource.options}
              value={selectedAssignee}
              onChange={(v) => updateAndRun({ assigneeId: v?.value ?? '' })}
              allowCustomValue
              placeholder="Any assignee"
              noOptionsMessage="No admins found"
            />
          </InlineField>
          <InlineField
            label="Team"
            labelWidth={12}
            tooltip="Filter by team assignee (team_assignee_id)."
            error={teamsResource.error}
            invalid={!!teamsResource.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={teamsResource.loading}
              options={teamsResource.options}
              value={selectedTeam}
              onChange={(v) => updateAndRun({ teamId: v?.value ?? '' })}
              allowCustomValue
              placeholder="Any team"
              noOptionsMessage="No teams found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {isSearchable && (
        <InlineFieldRow>
          <InlineField
            label="Tag"
            labelWidth={LABEL_WIDTH}
            tooltip="Filter by tag (tag_ids contains)."
            error={tagsResource.error}
            invalid={!!tagsResource.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={tagsResource.loading}
              options={tagsResource.options}
              value={selectedTag}
              onChange={(v) => updateAndRun({ tagId: v?.value ?? '' })}
              allowCustomValue
              placeholder="Any tag"
              noOptionsMessage="No tags found"
            />
          </InlineField>
          <InlineField label="Search" labelWidth={12} tooltip="Free-text contains match against the entity's primary text field (e.g. email for contacts).">
            <Input
              width={INPUT_WIDTH}
              value={searchQuery ?? ''}
              placeholder="contains…"
              onChange={onInputChange('searchQuery')}
              onBlur={onRunQuery}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {isSearchable && (
        <InlineFieldRow>
          <InlineField
            label="Filters"
            labelWidth={LABEL_WIDTH}
            tooltip="Intercom Search API conditions. Multiple rows are combined with AND."
          >
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              {currentFilters.map((f, i) => (
                <div key={i} style={{ display: 'flex', gap: '4px', alignItems: 'center' }}>
                  {i > 0 && <span style={{ fontWeight: 'bold', marginRight: '4px' }}>AND</span>}
                  <Input
                    width={22}
                    value={f.field}
                    placeholder="field (e.g. created_at)"
                    onChange={(e: ChangeEvent<HTMLInputElement>) => updateFilter(i, { field: e.target.value })}
                    onBlur={onRunQuery}
                  />
                  <Select<string>
                    width={16}
                    options={operatorOptions()}
                    value={f.operator}
                    onChange={(v) => v?.value && updateFilter(i, { operator: v.value })}
                  />
                  <Input
                    width={20}
                    value={f.value}
                    placeholder="value"
                    onChange={(e: ChangeEvent<HTMLInputElement>) => updateFilter(i, { value: e.target.value })}
                    onBlur={onRunQuery}
                  />
                  <button
                    className="gf-form-label gf-form-label--btn"
                    onClick={() => removeFilter(i)}
                    title="Remove filter"
                    style={{ cursor: 'pointer', padding: '0 4px' }}
                  >
                    x
                  </button>
                </div>
              ))}
              <button
                className="gf-form-label gf-form-label--btn"
                onClick={addFilter}
                style={{ cursor: 'pointer', alignSelf: 'flex-start', marginTop: '2px' }}
              >
                + Add filter
              </button>
            </div>
          </InlineField>
        </InlineFieldRow>
      )}

      {showSortLimit && (
        <>
          <InlineFieldRow>
            <InlineField label="Sort by" labelWidth={LABEL_WIDTH} tooltip="Field to sort results by (e.g. created_at).">
              <Input
                width={INPUT_WIDTH}
                value={sortField}
                placeholder="created_at"
                onChange={onSortFieldChange}
                onBlur={onRunQuery}
              />
            </InlineField>
            <InlineField label="Direction" labelWidth={12} tooltip="Sort direction.">
              <RadioButtonGroup<SortDirection> options={SORT_DIR_OPTIONS} value={sortDir} onChange={onSortDirChange} />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Limit" labelWidth={LABEL_WIDTH} tooltip="Maximum records to return. 0 returns all (auto-paginated up to a safety cap).">
              <Input
                width={20}
                type="number"
                min={0}
                value={limit ?? 0}
                onChange={(e: ChangeEvent<HTMLInputElement>) => {
                  const n = parseInt(e.target.value, 10);
                  update({ limit: isNaN(n) ? 0 : n });
                }}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>
        </>
      )}
    </div>
  );
}
