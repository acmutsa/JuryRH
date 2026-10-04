import { useEffect, useRef, useState } from 'react';
import Button from '../Button';
import { putRequest } from '../../api';
import { errorAlert } from '../../util';
import { useChallengePicksStore } from '../../store';

interface ChallengePicksProps {
    /* Limit review to the challenge being replaced during judging. */
    challenge?: string;
    /* Queue a replacement to save together with the current project's submission. */
    onReplace?: (challenge: string, previous: ChallengePickProject) => void;
    /* Optional layout overrides. */
    className?: string;
}

interface PickChange {
    challenge: string;
    target: ChallengePickProject;
    previous?: ChallengePickProject;
    starred: boolean;
    choosePrevious?: boolean;
}

const ChallengePicks = (props: ChallengePicksProps) => {
    const { picks, loading, error, fetchPicks } = useChallengePicksStore();
    const [change, setChange] = useState<PickChange | null>(null);
    const confirmationRef = useRef<HTMLDivElement>(null);
    const [saving, setSaving] = useState(false);

    useEffect(() => {
        fetchPicks();
    }, [props.challenge, fetchPicks]);

    useEffect(() => {
        if (change) confirmationRef.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    }, [change]);

    const save = async () => {
        if (!change || saving) return;
        if (props.onReplace) {
            props.onReplace(change.challenge, change.target);
            setChange(null);
            return;
        }
        setSaving(true);
        const res = await putRequest<OkResponse>('/judge/challenge-picks', 'judge', {
            challenge: change.challenge,
            project_id: change.target.id,
            starred: change.starred,
            replace_project_id: change.previous?.id,
        });
        if (res.status !== 200) {
            errorAlert(res);
        }
        setChange(null);
        await fetchPicks();
        setSaving(false);
    };

    if (loading) return <p role="status">Loading your challenge picks…</p>;
    if (error) {
        return (
            <div role="alert">
                <p>{error}</p>
                <Button type="outline" onClick={fetchPicks}>Retry</Button>
            </div>
        );
    }
    if (!picks) return null;
    const groups = picks.challenges.filter((group) => !props.challenge || group.name === props.challenge);
    const previousPicks = picks.challenges
        .find((group) => group.name === change?.challenge)
        ?.projects.filter((project) => project.starred) || [];
    const confirmation = !change
        ? ''
        : props.onReplace
          ? `Move star from ${change.target.name} to this project? It will save when you submit.`
          : change.previous
            ? `Move star from ${change.previous.name} to ${change.target.name}?`
            : `${change.starred ? 'Add' : 'Remove'} star ${change.starred ? 'for' : 'from'} ${change.target.name}?`;

    return (
        <div className={props.className}>
            <h3 className="font-bold">My Challenge Picks</h3>
            <p className="text-sm text-light">Your picks are provisional until judging closes.</p>
            {picks.locked && <p className="text-sm">Judging has closed. Your picks are locked.</p>}
            {groups.length === 0 && <p className="text-sm">No challenges are enabled for stars.</p>}
            {groups.map((group) => (
                <section key={group.name} className="mt-3 text-left">
                    <h4 className="font-bold break-words">{group.name}</h4>
                    <p className="text-sm text-light">
                        {picks.limit - group.remaining} of {picks.limit} stars used · You've judged{' '}
                        {group.judged} of {group.total} eligible projects
                    </p>
                    {group.projects.length === 0 && (
                        <p className="text-sm">You haven't judged an eligible project yet.</p>
                    )}
                    {props.onReplace && !group.projects.some((project) => project.starred) && (
                        <p className="text-sm">You have no picks to replace in this challenge yet.</p>
                    )}
                    {group.projects
                        .filter((project) => !props.onReplace || project.starred)
                        .sort((a, b) => Number(b.starred) - Number(a.starred) || a.location - b.location)
                        .map((project) => (
                            <div key={project.id} className="py-2 border-b border-lightest">
                                <p className="break-words">
                                    {project.starred ? '★ ' : ''}{project.name} · Table {project.location}
                                </p>
                                <Button
                                    type="outline"
                                    className="mt-1 min-h-11 text-sm break-words max-w-full"
                                    disabled={picks.locked || saving}
                                    onClick={() => setChange({
                                        challenge: group.name,
                                        target: project,
                                        starred: !project.starred,
                                        choosePrevious: !props.onReplace && !project.starred && group.remaining === 0,
                                    })}
                                >
                                    {props.onReplace
                                        ? 'Replace this pick'
                                        : project.starred
                                          ? 'Remove star'
                                          : group.remaining === 0
                                            ? 'Replace a pick'
                                            : 'Add star'}
                                </Button>
                            </div>
                        ))}
                </section>
            ))}
            {change && (
                <div
                    className="mt-3 p-3 border border-primary rounded text-left"
                    role="region"
                    aria-label="Confirm challenge pick"
                    ref={confirmationRef}
                    aria-live="polite"
                >
                    {change.choosePrevious ? (
                        <>
                            <p className="break-words">Choose a pick to replace with {change.target.name}:</p>
                            {previousPicks.map((project) => (
                                <Button
                                    key={project.id}
                                    type="outline"
                                    className="mt-2 min-h-11 text-sm break-words max-w-full"
                                    onClick={() => setChange({
                                        ...change,
                                        previous: project,
                                        choosePrevious: false,
                                    })}
                                >
                                    {project.name} · Table {project.location}
                                </Button>
                            ))}
                        </>
                    ) : (
                        <>
                            <p className="break-words">{confirmation}</p>
                            <Button
                                type="primary"
                                className="mt-2 min-h-11 text-sm"
                                onClick={save}
                                disabled={saving || picks.locked}
                            >
                                Confirm
                            </Button>
                        </>
                    )}
                    <Button
                        type="outline"
                        className="mt-2 min-h-11 text-sm"
                        onClick={() => setChange(null)}
                        disabled={saving}
                    >
                        Cancel
                    </Button>
                </div>
            )}
        </div>
    );
};

export default ChallengePicks;
