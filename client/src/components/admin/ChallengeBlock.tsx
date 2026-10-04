import { useCallback, useEffect, useState } from 'react';
import { getRequest, postRequest } from '../../api';
import { useOptionsStore } from '../../store';
import { errorAlert } from '../../util';

interface ChallengeNomination {
    project_id: string;
    name: string;
    location: number;
    stars: number;
}

const ChallengeBlock = () => {
    const [challenges, setChallenges] = useState<string[]>([]);
    const [nominations, setNominations] = useState<Record<string, ChallengeNomination[]>>({});
    const [saving, setSaving] = useState(false);
    const options = useOptionsStore((state) => state.options);
    const fetchOptions = useOptionsStore((state) => state.fetchOptions);

    const refreshNominations = useCallback(async () => {
        const res = await getRequest<Record<string, ChallengeNomination[]>>(
            '/admin/challenge-nominations',
            'admin'
        );
        if (res.status !== 200) errorAlert(res);
        else setNominations(res.data ?? {});
    }, []);

    useEffect(() => {
        async function fetchData() {
            const challengesRes = await getRequest<string[]>('/challenges', '');
            if (challengesRes.status !== 200) errorAlert(challengesRes);
            else setChallenges((challengesRes.data ?? []).slice().sort());
            await refreshNominations();
        }

        fetchData();
    }, [refreshNominations]);

    const toggle = async (challenge: string) => {
        if (saving) return;
        const enabled = options.opt_in_challenges ?? [];
        const next = enabled.includes(challenge)
            ? enabled.filter((name) => name !== challenge)
            : [...enabled, challenge];
        setSaving(true);
        const res = await postRequest<OkResponse>('/admin/options', 'admin', {
            opt_in_challenges: next,
        });
        if (res.status !== 200) errorAlert(res);
        else await fetchOptions();
        setSaving(false);
    };

    if (challenges.length === 0) {
        return (
            <p className="text-sm text-light my-2">
                No challenges found. Add projects with challenge entries to enable challenge stars.
            </p>
        );
    }

    return (
        <div className="border-2 border-primary bg-primary/10 rounded-md p-3 my-2">
            <h1 className="text-lg md:text-xl font-bold">Challenge List</h1>
            <p className="text-sm text-light mb-2">
                Enable a challenge to let general judges star eligible projects when finishing.
            </p>
            <button
                type="button"
                onClick={refreshNominations}
                className="text-sm text-primary underline mb-2"
            >
                Refresh challenge stars
            </button>
            <ul className="space-y-3">
                {challenges.map((challenge) => {
                    const enabled = (options.opt_in_challenges ?? []).includes(challenge);
                    const projects = nominations[challenge] ?? [];
                    return (
                        <li key={challenge} className="border-t border-primary/30 pt-2">
                            <div className="flex items-center justify-between gap-3">
                                <span className="font-medium">{challenge}</span>
                                <button
                                    type="button"
                                    role="switch"
                                    aria-label={`Allow stars for ${challenge}`}
                                    aria-checked={enabled}
                                    disabled={saving}
                                    onClick={() => toggle(challenge)}
                                    className={`rounded-full px-3 py-1 text-sm min-w-14 ${
                                        enabled
                                            ? 'bg-primary text-white'
                                            : 'bg-backgroundDark text-light'
                                    }`}
                                >
                                    {enabled ? 'On' : 'Off'}
                                </button>
                            </div>
                            {projects.length > 0 && (
                                <ul className="text-sm text-light mt-1 space-y-1">
                                    {projects.map((project) => (
                                        <li key={project.project_id}>
                                            #{project.location} {project.name} — {project.stars}{' '}
                                            {project.stars === 1 ? 'star' : 'stars'}
                                        </li>
                                    ))}
                                </ul>
                            )}
                        </li>
                    );
                })}
            </ul>
        </div>
    );
};

export default ChallengeBlock;
